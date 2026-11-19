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

package filesystemconfig

import (
	"testing"

	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"

	svcapitypes "github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/apis/v1alpha1"
)

func TestNormalize_DropsUnmodelled(t *testing.T) {
	efs := &svcapitypes.FilesystemConfiguration{
		EFSAccessPoint: &svcapitypes.EFSAccessPointConfiguration{
			AccessPointARN: aws.String("arn:aws:elasticfilesystem:us-west-2:111122223333:access-point/fsap-0123456789abcdef0"),
			MountPath:      aws.String("/mnt/efs"),
		},
	}
	s3 := &svcapitypes.FilesystemConfiguration{
		S3FilesAccessPoint: &svcapitypes.S3FilesAccessPointConfiguration{
			AccessPointARN: aws.String("arn:aws:s3files:us-west-2:111122223333:file-system/fs-0123456789abcdef0/access-point/fsap-0123456789abcdef0"),
			MountPath:      aws.String("/mnt/s3"),
		},
	}
	session := &svcapitypes.FilesystemConfiguration{
		SessionStorage: &svcapitypes.SessionStorageConfiguration{MountPath: aws.String("/mnt/session")},
	}
	unmodelled := &svcapitypes.FilesystemConfiguration{}

	cases := []struct {
		name string
		in   []*svcapitypes.FilesystemConfiguration
		want []*svcapitypes.FilesystemConfiguration
	}{
		{"nil", nil, nil},
		{"empty", []*svcapitypes.FilesystemConfiguration{}, nil},
		{"only unmodelled", []*svcapitypes.FilesystemConfiguration{unmodelled, nil}, nil},
		{"all modelled kept in order", []*svcapitypes.FilesystemConfiguration{s3, efs, session}, []*svcapitypes.FilesystemConfiguration{s3, efs, session}},
		{"unmodelled dropped, order kept", []*svcapitypes.FilesystemConfiguration{efs, unmodelled, session}, []*svcapitypes.FilesystemConfiguration{efs, session}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Normalize(tc.in, nil))
		})
	}
}

func TestNormalize_RestoresAccessPointRefByMountPath(t *testing.T) {
	ref := func(name string) *ackv1alpha1.AWSResourceReferenceWrapper {
		return &ackv1alpha1.AWSResourceReferenceWrapper{From: &ackv1alpha1.AWSResourceReference{Name: aws.String(name)}}
	}
	efs := func(arn, mount string) *svcapitypes.FilesystemConfiguration {
		return &svcapitypes.FilesystemConfiguration{EFSAccessPoint: &svcapitypes.EFSAccessPointConfiguration{
			AccessPointARN: aws.String(arn), MountPath: aws.String(mount),
		}}
	}
	declaredEFS := func(r *ackv1alpha1.AWSResourceReferenceWrapper, arn, mount string) *svcapitypes.FilesystemConfiguration {
		fc := efs(arn, mount)
		fc.EFSAccessPoint.AccessPointRef = r
		return fc
	}

	// The service returns the entries in a different order and adds an
	// unmodelled member. The entry declared with an ARN keeps no ref, and
	// the declared ref is copied, not aliased.
	declared := []*svcapitypes.FilesystemConfiguration{
		declaredEFS(ref("ap-a"), "arn:a", "/mnt/a"),
		efs("arn:b", "/mnt/b"),
	}
	observed := []*svcapitypes.FilesystemConfiguration{
		efs("arn:b", "/mnt/b"),
		{},
		efs("arn:a", "/mnt/a"),
	}
	got := Normalize(observed, declared)
	assert.Len(t, got, 2)
	assert.Nil(t, got[0].EFSAccessPoint.AccessPointRef)
	assert.Equal(t, "ap-a", *got[1].EFSAccessPoint.AccessPointRef.From.Name)
	assert.NotSame(t, declared[0].EFSAccessPoint.AccessPointRef, got[1].EFSAccessPoint.AccessPointRef)

	// A mount path that is no longer declared gets no ref.
	got = Normalize([]*svcapitypes.FilesystemConfiguration{efs("arn:a", "/mnt/old")}, declared)
	assert.Nil(t, got[0].EFSAccessPoint.AccessPointRef)

	// A ref already on the observed entry is left alone.
	observedWithRef := []*svcapitypes.FilesystemConfiguration{declaredEFS(ref("other"), "arn:a", "/mnt/a")}
	got = Normalize(observedWithRef, declared)
	assert.Equal(t, "other", *got[0].EFSAccessPoint.AccessPointRef.From.Name)
}
