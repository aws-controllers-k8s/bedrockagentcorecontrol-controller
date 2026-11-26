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
	"io"
	"net/http"
	"strings"
	"testing"

	svcapitypes "github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/apis/v1alpha1"
	ackmetrics "github.com/aws-controllers-k8s/runtime/pkg/metrics"
	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"
	svcsdktypes "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cannedResponse answers every request with a fixed 200 JSON body.
type cannedResponse string

func (c cannedResponse) Do(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(string(c))),
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
			HTTPClient: cannedResponse(`{"agentRuntimeId": "rt-1", "filesystemConfigurations": [
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

func TestCapacityProviderConfigurationRoundTrip(t *testing.T) {
	const providerARN = "arn:aws:bedrock-agentcore:us-west-2:111122223333:capacity-provider/example-a1b2c3d4e5"

	for _, tc := range []struct {
		name       string
		response   cannedResponse
		spec       *svcapitypes.CapacityProviderConfiguration
		sdkPayload *svcsdktypes.CapacityProviderConfiguration
	}{
		{
			name:       "Instances runtime",
			response:   cannedResponse(`{"agentRuntimeId":"rt-1","capacityProviderConfiguration":{"capacityProviderArn":"` + providerARN + `"}}`),
			spec:       &svcapitypes.CapacityProviderConfiguration{CapacityProviderARN: aws.String(providerARN)},
			sdkPayload: &svcsdktypes.CapacityProviderConfiguration{CapacityProviderArn: aws.String(providerARN)},
		},
		{
			name:     "microVM runtime",
			response: cannedResponse(`{"agentRuntimeId":"rt-1"}`),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			rm := &resourceManager{
				sdkapi: svcsdk.NewFromConfig(aws.Config{
					Region:           "us-west-2",
					Credentials:      aws.AnonymousCredentials{},
					RetryMaxAttempts: 1,
					HTTPClient:       tc.response,
				}),
				metrics: ackmetrics.NewMetrics("bedrockagentcorecontrol"),
			}
			desired := &resource{ko: &svcapitypes.AgentRuntime{}}
			desired.ko.Status.AgentRuntimeID = aws.String("rt-1")
			desired.ko.Spec.CapacityProviderConfiguration = &svcapitypes.CapacityProviderConfiguration{
				CapacityProviderARN: aws.String("arn:aws:bedrock-agentcore:us-west-2:111122223333:capacity-provider/stale-a1b2c3d4e5"),
			}

			observed, err := rm.sdkFind(ctx, desired)
			require.NoError(t, err)
			assert.Equal(t, tc.spec, observed.ko.Spec.CapacityProviderConfiguration)

			desired.ko.Spec.CapacityProviderConfiguration = tc.spec
			create, err := rm.newCreateRequestPayload(ctx, desired)
			require.NoError(t, err)
			assert.Equal(t, tc.sdkPayload, create.CapacityProviderConfiguration)
			assert.Nil(t, create.NetworkConfiguration)

			// An unrelated update must preserve the selected compute configuration
			desired.ko.Spec.Description = aws.String("new description")
			delta := newResourceDelta(desired, observed)
			assert.True(t, delta.DifferentAt("Spec.Description"))
			assert.False(t, delta.DifferentAt("Spec.CapacityProviderConfiguration"))
			update, err := rm.newUpdateRequestPayload(ctx, desired, delta)
			require.NoError(t, err)
			assert.Equal(t, tc.sdkPayload, update.CapacityProviderConfiguration)
			assert.Nil(t, update.NetworkConfiguration)
		})
	}
}

// updateHTTPClient captures requests sent by the AWS SDK during updates.
type updateHTTPClient func(*http.Request) (*http.Response, error)

func (client updateHTTPClient) Do(request *http.Request) (*http.Response, error) {
	return client(request)
}

// TestCapacityProviderConfigurationUpdates exercises the update gate and payload.
func TestCapacityProviderConfigurationUpdates(t *testing.T) {
	const (
		observedARN = "arn:aws:bedrock-agentcore:us-west-2:111122223333:capacity-provider/original-a1b2c3d4e5"
		newARN      = "arn:aws:bedrock-agentcore:us-west-2:111122223333:capacity-provider/replacement-a1b2c3d4e5"
		runtimeID   = "example-a1b2c3d4e5"
	)

	for _, tc := range []struct {
		name        string
		desiredARN  string
		description string
		deltaPath   string
		updateARN   string
	}{
		{
			name: "omit provider on an adopted runtime",
		},
		{
			name:        "preserve the adopted provider during an unrelated update",
			description: "updated description",
			deltaPath:   "Spec.Description",
			updateARN:   observedARN,
		},
		{
			name:       "change only the provider ARN",
			desiredARN: newARN,
			deltaPath:  "Spec.CapacityProviderConfiguration.CapacityProviderARN",
			updateARN:  newARN,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			desired := &resource{ko: &svcapitypes.AgentRuntime{}}
			desired.ko.Status.AgentRuntimeID = aws.String(runtimeID)
			desired.ko.Status.Status = aws.String("READY")
			desired.ko.Spec.RoleARN = aws.String("arn:aws:iam::111122223333:role/BedrockAgentCoreRuntime")
			desired.ko.Spec.AgentRuntimeArtifact = &svcapitypes.AgentRuntimeArtifact{
				ContainerConfiguration: &svcapitypes.ContainerConfiguration{
					ContainerURI: aws.String("111122223333.dkr.ecr.us-west-2.amazonaws.com/example:latest"),
				},
			}
			observed := &resource{ko: desired.ko.DeepCopy()}
			observed.ko.Spec.CapacityProviderConfiguration = &svcapitypes.CapacityProviderConfiguration{
				CapacityProviderARN: aws.String(observedARN),
			}
			if tc.desiredARN != "" {
				desired.ko.Spec.CapacityProviderConfiguration = &svcapitypes.CapacityProviderConfiguration{
					CapacityProviderARN: aws.String(tc.desiredARN),
				}
			}
			if tc.description != "" {
				desired.ko.Spec.Description = aws.String(tc.description)
			}

			calls := 0
			client := updateHTTPClient(func(request *http.Request) (*http.Response, error) {
				calls++
				assert.Equal(t, http.MethodPut, request.Method)
				var body struct {
					CapacityProviderConfiguration *svcsdktypes.CapacityProviderConfiguration
					Description                   *string
				}
				require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
				require.NotNil(t, body.CapacityProviderConfiguration)
				assert.Equal(t, tc.updateARN, aws.ToString(body.CapacityProviderConfiguration.CapacityProviderArn))
				assert.Equal(t, tc.description, aws.ToString(body.Description))
				return cannedResponse(`{"agentRuntimeId":"` + runtimeID + `","status":"UPDATING"}`).Do(request)
			})
			rm := &resourceManager{
				sdkapi: svcsdk.NewFromConfig(aws.Config{
					Region:           "us-west-2",
					Credentials:      aws.AnonymousCredentials{},
					RetryMaxAttempts: 1,
					HTTPClient:       client,
				}),
				metrics: ackmetrics.NewMetrics("bedrockagentcorecontrol"),
			}

			delta := newResourceDelta(desired, observed)
			if tc.deltaPath == "" {
				assert.Empty(t, delta.Differences)
			} else {
				assert.Len(t, delta.Differences, 1)
				assert.True(t, delta.DifferentAt(tc.deltaPath))
			}
			_, err := rm.sdkUpdate(t.Context(), desired, observed, delta)
			require.NoError(t, err)
			if tc.updateARN == "" {
				assert.Zero(t, calls)
			} else {
				assert.Equal(t, 1, calls)
			}
			if tc.desiredARN == "" {
				assert.Nil(t, desired.ko.Spec.CapacityProviderConfiguration)
			}
		})
	}
}
