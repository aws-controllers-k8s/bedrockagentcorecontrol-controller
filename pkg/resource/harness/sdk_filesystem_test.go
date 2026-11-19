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

package harness

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackmetrics "github.com/aws-controllers-k8s/runtime/pkg/metrics"
	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	svcapitypes "github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/apis/v1alpha1"
)

const testAccessPointARN = "arn:aws:elasticfilesystem:us-west-2:111122223333:access-point/fsap-0123456789abcdef0"

// cannedResponse answers every request with a fixed 200 JSON body.
type cannedResponse string

func (c cannedResponse) Do(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(string(c))),
	}, nil
}

// harnessBody is a Get/Create/UpdateHarness response whose runtime
// environment reports the resolved EFS ARN (no ref) plus a union member the
// CRD does not model.
const harnessBody = `{"harness": {"harnessId": "h-1", "harnessName": "h", "status": "READY",
	"arn": "arn:aws:bedrock-agentcore:us-west-2:111122223333:harness/h-1",
	"environment": {"agentCoreRuntimeEnvironment": {"filesystemConfigurations": [
		{"sessionStorage": {"mountPath": "/mnt/session"}},
		{"efsAccessPoint": {"accessPointArn": "` + testAccessPointARN + `", "mountPath": "/mnt/efs"}},
		{"capacityProviderVolume": {"volumeName": "v", "mountPath": "/mnt/vol"}}]}}}}`

func newCannedRM() *resourceManager {
	return &resourceManager{
		sdkapi: svcsdk.NewFromConfig(aws.Config{
			Region:           "us-west-2",
			Credentials:      aws.AnonymousCredentials{},
			RetryMaxAttempts: 1,
			HTTPClient:       cannedResponse(harnessBody),
		}),
		metrics: ackmetrics.NewMetrics("bedrockagentcorecontrol"),
	}
}

// harnessWithEFSRef is a Harness whose EFS entry was declared through
// accessPointRef and has been resolved to the ARN, as the runtime hands it to
// sdkFind/sdkCreate/sdkUpdate.
func harnessWithEFSRef() *resource {
	return &resource{ko: &svcapitypes.Harness{
		Spec: svcapitypes.HarnessSpec{
			HarnessName:      aws.String("h"),
			ExecutionRoleARN: aws.String("arn:aws:iam::111122223333:role/harness"),
			Environment: &svcapitypes.HarnessEnvironmentProviderRequest{
				AgentCoreRuntimeEnvironment: &svcapitypes.HarnessAgentCoreRuntimeEnvironmentRequest{
					FilesystemConfigurations: []*svcapitypes.FilesystemConfiguration{
						{SessionStorage: &svcapitypes.SessionStorageConfiguration{MountPath: aws.String("/mnt/session")}},
						{EFSAccessPoint: &svcapitypes.EFSAccessPointConfiguration{
							AccessPointARN: aws.String(testAccessPointARN),
							AccessPointRef: &ackv1alpha1.AWSResourceReferenceWrapper{
								From: &ackv1alpha1.AWSResourceReference{Name: aws.String("ap")},
							},
							MountPath: aws.String("/mnt/efs"),
						}},
					},
				},
			},
		},
	}}
}

// assertConverged checks that a set-output path ran the normalize hook: the
// unmodelled member is gone, the ref survived, and nothing differs from what
// was declared.
func assertConverged(t *testing.T, desired, got *resource) {
	t.Helper()
	require.NotNil(t, got.ko.Spec.Environment)
	require.NotNil(t, got.ko.Spec.Environment.AgentCoreRuntimeEnvironment)
	fcs := got.ko.Spec.Environment.AgentCoreRuntimeEnvironment.FilesystemConfigurations
	require.Len(t, fcs, 2)
	require.NotNil(t, fcs[1].EFSAccessPoint)
	assert.NotNil(t, fcs[1].EFSAccessPoint.AccessPointRef)
	assert.False(t, newResourceDelta(desired, got).DifferentAt("Spec.Environment"))
}

// Each test goes through the generated sdk* method so that dropping the
// corresponding hook from generator.yaml fails it; calling the wrapper
// directly would not.
func TestSDKFind_FilesystemConfigurationsConverge(t *testing.T) {
	desired := harnessWithEFSRef()
	desired.ko.Status.HarnessID = aws.String("h-1")

	latest, err := newCannedRM().sdkFind(context.Background(), desired)
	require.NoError(t, err)
	assertConverged(t, desired, latest)
}

func TestSDKCreate_FilesystemConfigurationsConverge(t *testing.T) {
	desired := harnessWithEFSRef()

	created, err := newCannedRM().sdkCreate(context.Background(), desired)
	require.NoError(t, err)
	assertConverged(t, desired, created)
}

func TestSDKUpdate_FilesystemConfigurationsConverge(t *testing.T) {
	desired := harnessWithEFSRef()
	desired.ko.Status.HarnessID = aws.String("h-1")
	latest := harnessWithEFSRef()
	latest.ko.Status.HarnessID = aws.String("h-1")
	latest.ko.Status.Status = aws.String("READY")
	delta := ackcompare.NewDelta()
	delta.Add("Spec.Environment", desired.ko.Spec.Environment, nil)

	updated, err := newCannedRM().sdkUpdate(context.Background(), desired, latest, delta)
	require.NoError(t, err)
	assertConverged(t, desired, updated)
}
