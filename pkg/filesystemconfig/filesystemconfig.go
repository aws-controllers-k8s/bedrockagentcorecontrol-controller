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

// Package filesystemconfig holds helpers for the FilesystemConfiguration
// union shared by AgentRuntime and Harness.
package filesystemconfig

import (
	svcapitypes "github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/apis/v1alpha1"
)

// Normalize prepares a FilesystemConfigurations list that was rebuilt from an
// AWS response so it can be compared with, and patched over, the declared
// spec. It is called on every set-output path that rebuilds the list.
//
// It does two things:
//
//   - It drops the entries that the generated code returns empty because the
//     service sent a union member the CRD does not model (capacityProviderVolume,
//     which is ignored in generator.yaml, or a member newer than the pinned
//     SDK). Without this each one shows up in the spec as {} and produces a
//     delta that never converges. The list is returned as nil when every entry
//     is dropped, so it compares equal to an unset spec.
//
//   - It copies efsAccessPoint.accessPointRef from the declared entry with the
//     same mountPath. The generated EnsureReferences does not restore a
//     reference that is reached through a list, so without this the spec patch
//     after a read would delete the user's accessPointRef and store the
//     resolved ARN in its place. mountPath is the key because the service
//     requires it to be unique across the list, while the service may return
//     the entries in any order.
func Normalize(
	observed []*svcapitypes.FilesystemConfiguration,
	declared []*svcapitypes.FilesystemConfiguration,
) []*svcapitypes.FilesystemConfiguration {
	out := dropUnmodelled(observed)
	for _, o := range out {
		efs := o.EFSAccessPoint
		if efs == nil || efs.AccessPointRef != nil || efs.MountPath == nil {
			continue
		}
		for _, d := range declared {
			if d == nil || d.EFSAccessPoint == nil || d.EFSAccessPoint.AccessPointRef == nil {
				continue
			}
			if d.EFSAccessPoint.MountPath != nil && *d.EFSAccessPoint.MountPath == *efs.MountPath {
				efs.AccessPointRef = d.EFSAccessPoint.AccessPointRef.DeepCopy()
				break
			}
		}
	}
	return out
}

func dropUnmodelled(
	in []*svcapitypes.FilesystemConfiguration,
) []*svcapitypes.FilesystemConfiguration {
	if in == nil {
		return nil
	}
	out := make([]*svcapitypes.FilesystemConfiguration, 0, len(in))
	for _, fc := range in {
		if fc == nil {
			continue
		}
		if fc.EFSAccessPoint == nil && fc.S3FilesAccessPoint == nil && fc.SessionStorage == nil {
			continue
		}
		out = append(out, fc)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
