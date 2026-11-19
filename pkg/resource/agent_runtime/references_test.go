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
	"testing"

	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	svcapitypes "github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/apis/v1alpha1"
)

func runtimeWithEFS(ref *ackv1alpha1.AWSResourceReferenceWrapper, arn *string) *resource {
	return &resource{ko: &svcapitypes.AgentRuntime{
		Spec: svcapitypes.AgentRuntimeSpec{
			FilesystemConfigurations: []*svcapitypes.FilesystemConfiguration{
				{SessionStorage: &svcapitypes.SessionStorageConfiguration{MountPath: aws.String("/mnt/session")}},
				{EFSAccessPoint: &svcapitypes.EFSAccessPointConfiguration{
					AccessPointARN: arn,
					AccessPointRef: ref,
					MountPath:      aws.String("/mnt/efs"),
				}},
			},
		},
	}}
}

func efsRef(name string) *ackv1alpha1.AWSResourceReferenceWrapper {
	return &ackv1alpha1.AWSResourceReferenceWrapper{From: &ackv1alpha1.AWSResourceReference{Name: aws.String(name)}}
}

func TestValidateReferenceFields_EFSAccessPoint(t *testing.T) {
	arn := aws.String("arn:aws:elasticfilesystem:us-west-2:111122223333:access-point/fsap-0123456789abcdef0")

	assert.NoError(t, validateReferenceFields(runtimeWithEFS(nil, arn).ko))
	assert.NoError(t, validateReferenceFields(runtimeWithEFS(efsRef("ap"), nil).ko))
	assert.Error(t, validateReferenceFields(runtimeWithEFS(efsRef("ap"), arn).ko))
}

func TestClearResolvedReferences_EFSAccessPoint(t *testing.T) {
	rm := &resourceManager{}
	arn := aws.String("arn:aws:elasticfilesystem:us-west-2:111122223333:access-point/fsap-0123456789abcdef0")

	// A resolved ARN is cleared when its ref is present.
	cleared := rm.ClearResolvedReferences(runtimeWithEFS(efsRef("ap"), arn)).(*resource)
	efs := cleared.ko.Spec.FilesystemConfigurations[1].EFSAccessPoint
	assert.Nil(t, efs.AccessPointARN)
	require.NotNil(t, efs.AccessPointRef)
	assert.Equal(t, "ap", *efs.AccessPointRef.From.Name)

	// A declared ARN without a ref is kept.
	kept := rm.ClearResolvedReferences(runtimeWithEFS(nil, arn)).(*resource)
	assert.Equal(t, *arn, *kept.ko.Spec.FilesystemConfigurations[1].EFSAccessPoint.AccessPointARN)
}

// ReadOne rebuilds Spec.FilesystemConfigurations from GetAgentRuntime, which
// drops accessPointRef, and the generated EnsureReferences does not restore a
// ref reached through a list. The read hook must put it back, or the spec
// patch would replace the user's ref with the resolved ARN.
func TestNormalizeFilesystemConfigurations_KeepsRefAcrossRead(t *testing.T) {
	arn := aws.String("arn:aws:elasticfilesystem:us-west-2:111122223333:access-point/fsap-0123456789abcdef0")
	resolved := runtimeWithEFS(efsRef("ap"), arn)
	observed := runtimeWithEFS(nil, arn)
	// The service returns an unmodelled member as well (capacityProviderVolume).
	observed.ko.Spec.FilesystemConfigurations = append(observed.ko.Spec.FilesystemConfigurations, &svcapitypes.FilesystemConfiguration{})

	normalizeFilesystemConfigurations(observed.ko, resolved.ko)

	require.Len(t, observed.ko.Spec.FilesystemConfigurations, 2)
	cleared := (&resourceManager{}).ClearResolvedReferences(observed).(*resource)
	assert.Equal(t,
		(&resourceManager{}).ClearResolvedReferences(resolved).(*resource).ko.Spec.FilesystemConfigurations,
		cleared.ko.Spec.FilesystemConfigurations,
	)
}
