// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package agent_runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	svcapitypes "github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/apis/v1alpha1"
	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	ackmetrics "github.com/aws-controllers-k8s/runtime/pkg/metrics"
	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"
)

// Exercise the real generated read/compare/update path through the AWS SDK.
// The former sdkFind copied desired tags, making every tag delta disappear.
func TestRuntimeTagReadbackAndReconciliation(t *testing.T) {
	for _, mode := range []string{"changed", "added", "removed", "empty", "list-denied", "tag-denied", "untag-denied"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			arn := ackv1alpha1.AWSResourceName("arn:aws:bedrock-agentcore:us-west-2:123456789012:runtime/probe")
			wanted := map[string]string{"maxactiveruns": "4", "eks:controller-version": "bedrockagentcorecontrol-v1.20.0", "aws:cloudformation:stack-name": "retained"}
			actual := map[string]string{"maxactiveruns": "1", "eks:controller-version": wanted["eks:controller-version"], "aws:cloudformation:stack-name": "retained"}
			switch mode {
			case "added":
				delete(actual, "maxactiveruns")
			case "removed", "untag-denied":
				actual["obsolete"] = "remove"
			case "empty":
				actual = map[string]string{}
				wanted = map[string]string{"maxactiveruns": "4"}
			}
			listCalls, mutations := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if !strings.Contains(r.URL.Path, "/tags/") {
					if r.Method != http.MethodGet {
						t.Errorf("tag-only update invoked runtime configuration write: %s", r.Method)
					}
					json.NewEncoder(w).Encode(map[string]any{"agentRuntimeArn": string(arn), "agentRuntimeId": "probe", "agentRuntimeVersion": "2", "status": "READY"})
					return
				}
				if !strings.Contains(r.URL.Path, string(arn)) {
					t.Errorf("tags requested for wrong ARN: %s", r.URL.Path)
				}
				denied := (mode == "list-denied" && r.Method == http.MethodGet) || (mode == "tag-denied" && r.Method == http.MethodPost) || (mode == "untag-denied" && r.Method == http.MethodDelete)
				if denied {
					w.WriteHeader(http.StatusForbidden)
					w.Write([]byte(`{"__type":"AccessDeniedException","message":"denied"}`))
					return
				}
				switch r.Method {
				case http.MethodGet:
					listCalls++
					json.NewEncoder(w).Encode(map[string]any{"tags": actual})
				case http.MethodPost:
					mutations++
					var input struct {
						Tags map[string]string `json:"tags"`
					}
					if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
						t.Error(err)
					}
					for key, value := range input.Tags {
						if (strings.HasPrefix(key, "aws:") || strings.HasPrefix(key, "eks:")) && mode != "empty" {
							t.Errorf("rewrote retained system tag %s", key)
						}
						actual[key] = value
					}
					w.Write([]byte(`{}`))
				case http.MethodDelete:
					mutations++
					for _, key := range r.URL.Query()["tagKeys"] {
						if strings.HasPrefix(key, "aws:") || strings.HasPrefix(key, "eks:") {
							t.Errorf("deleted system tag %s", key)
						}
						delete(actual, key)
					}
					w.Write([]byte(`{}`))
				default:
					t.Errorf("unexpected tag method %s", r.Method)
				}
			}))
			defer server.Close()
			rm := &resourceManager{metrics: ackmetrics.NewMetrics("runtime-tag-test"), sdkapi: svcsdk.NewFromConfig(aws.Config{Region: "us-west-2", Credentials: aws.AnonymousCredentials{}, RetryMaxAttempts: 1}, func(o *svcsdk.Options) { o.BaseEndpoint = aws.String(server.URL) })}
			desired := &resource{ko: &svcapitypes.AgentRuntime{}}
			desired.ko.Spec.Tags = aws.StringMap(wanted)
			desired.ko.Status.AgentRuntimeID = aws.String("probe")
			desired.ko.Status.AgentRuntimeVersion = aws.String("2")
			latest, err := rm.sdkFind(ctx, desired)
			if mode == "list-denied" {
				if err == nil || latest != nil {
					t.Fatal("tag read failure must not yield an observed resource")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(aws.ToStringMap(latest.ko.Spec.Tags), actual) {
				t.Fatal("observed tags did not come from AWS")
			}
			if !reflect.DeepEqual(aws.ToStringMap(desired.ko.Spec.Tags), wanted) {
				t.Fatal("read mutated desired tags")
			}
			// The manager mirrors immutable AWS tags before comparing.
			mirrorAWSTags(desired, latest)
			delta := newResourceDelta(desired, latest)
			if !delta.DifferentAt("Spec.Tags") || delta.DifferentExcept("Spec.Tags") {
				t.Fatalf("expected only a tag delta: %v", delta.Differences)
			}
			_, err = rm.sdkUpdate(ctx, desired, latest, delta)
			if mode == "tag-denied" || mode == "untag-denied" {
				if err == nil {
					t.Fatal("tag write failure was swallowed")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(actual, wanted) {
				t.Fatalf("tags not reconciled: %#v", actual)
			}
			latest, err = rm.sdkFind(ctx, desired)
			if err != nil {
				t.Fatal(err)
			}
			if len(newResourceDelta(desired, latest).Differences) != 0 {
				t.Fatal("retry still sees a delta after reconciliation")
			}
			if listCalls != 2 || mutations == 0 {
				t.Fatalf("expected independent tag reads and writes: reads=%d writes=%d", listCalls, mutations)
			}
		})
	}
}
