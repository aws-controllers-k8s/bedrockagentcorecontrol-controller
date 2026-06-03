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

package code_interpreter

import (
	"context"
	"fmt"

	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackerr "github.com/aws-controllers-k8s/runtime/pkg/errors"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"

	"github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/pkg/tags"
)

// getTags retrieves the tags for a given resource ARN using the
// ListTagsForResource API and returns them as a map of string pointers.
func (rm *resourceManager) getTags(
	ctx context.Context,
	resourceARN string,
) (map[string]*string, error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.getTags")
	defer func() { exit(nil) }()

	resp, err := rm.sdkapi.ListTagsForResource(ctx, &svcsdk.ListTagsForResourceInput{
		ResourceArn: &resourceARN,
	})
	rm.metrics.RecordAPICall("GET", "ListTagsForResource", err)
	if err != nil {
		return nil, err
	}

	return aws.StringMap(resp.Tags), nil
}

// syncTags keeps the resource's tags in sync by calling the TagResource and
// UntagResource APIs.
func (rm *resourceManager) syncTags(
	ctx context.Context,
	desired *resource,
	latest *resource,
) error {
	resourceARN := string(*latest.ko.Status.ACKResourceMetadata.ARN)
	desiredTags := aws.ToStringMap(desired.ko.Spec.Tags)
	existingTags := aws.ToStringMap(latest.ko.Spec.Tags)

	return tags.SyncTags(
		ctx,
		rm.sdkapi,
		rm.metrics,
		resourceARN,
		desiredTags,
		existingTags,
	)
}

// customUpdate handles updates to the CodeInterpreter resource. Since there is
// no Update API, only tag changes are supported.
func (rm *resourceManager) customUpdate(
	ctx context.Context,
	desired *resource,
	latest *resource,
	delta *ackcompare.Delta,
) (*resource, error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.customUpdate")
	defer func(err error) { exit(err) }(nil)

	ko := desired.ko.DeepCopy()
	ko.Status = latest.ko.Status
	rm.setStatusDefaults(ko)

	if delta.DifferentAt("Spec.Tags") {
		if err := rm.syncTags(ctx, desired, latest); err != nil {
			return &resource{ko}, err
		}
	}

	if delta.DifferentExcept("Spec.Tags") {
		return nil, ackerr.NewTerminalError(fmt.Errorf("updating fields other than spec.tags is not supported."))
	}

	return &resource{ko}, nil
}
