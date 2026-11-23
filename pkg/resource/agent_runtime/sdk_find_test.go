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
	"io"
	"net/http"
	"strings"
	"testing"

	ackmetrics "github.com/aws-controllers-k8s/runtime/pkg/metrics"
	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cannedResponse returns the runtime body and an empty tag readback.
type cannedResponse string

func (c cannedResponse) Do(r *http.Request) (*http.Response, error) {
	body := string(c)
	if strings.HasPrefix(r.URL.Path, "/tags/") {
		body = `{"tags": {}}`
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

// sdkFind must run the read hook. Without it the observed list loses
// accessPointRef and keeps an empty entry for capacityProviderVolume, so
// every reconcile sends UpdateAgentRuntime. Calling the hook directly (as
// TestNormalizeFilesystemConfigurations_KeepsRefAcrossRead does) cannot catch
// the hook being dropped from generator.yaml.
func TestSDKFind_FilesystemConfigurationsConverge(t *testing.T) {
	arn := "arn:aws:elasticfilesystem:us-west-2:111122223333:access-point/fsap-0123456789abcdef0"
	rm := &resourceManager{
		sdkapi: svcsdk.NewFromConfig(aws.Config{
			Region:           "us-west-2",
			Credentials:      aws.AnonymousCredentials{},
			RetryMaxAttempts: 1,
			HTTPClient: cannedResponse(`{"agentRuntimeId": "rt-1", "agentRuntimeArn": "arn:aws:bedrock-agentcore:us-west-2:111122223333:runtime/rt-1", "filesystemConfigurations": [
				{"sessionStorage": {"mountPath": "/mnt/session"}},
				{"efsAccessPoint": {"accessPointArn": "` + arn + `", "mountPath": "/mnt/efs"}},
				{"capacityProviderVolume": {"volumeName": "v", "mountPath": "/mnt/vol"}}]}`),
		}),
		metrics: ackmetrics.NewMetrics("bedrockagentcorecontrol"),
	}
	desired := runtimeWithEFS(efsRef("ap"), aws.String(arn))
	desired.ko.Status.AgentRuntimeID = aws.String("rt-1")

	latest, err := rm.sdkFind(context.Background(), desired)
	require.NoError(t, err)

	require.Len(t, latest.ko.Spec.FilesystemConfigurations, 2)
	assert.NotNil(t, latest.ko.Spec.FilesystemConfigurations[1].EFSAccessPoint.AccessPointRef)
	assert.False(t, newResourceDelta(desired, latest).DifferentAt("Spec.FilesystemConfigurations"))
}
