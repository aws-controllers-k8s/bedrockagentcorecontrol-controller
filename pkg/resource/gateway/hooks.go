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

package gateway

import (
	"context"
	"fmt"
	"time"

	ackrequeue "github.com/aws-controllers-k8s/runtime/pkg/requeue"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"
	svcsdktypes "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol/types"

	"github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/pkg/tags"
)

var syncTags = tags.SyncTags

var (
	requeueNotReady = ackrequeue.NeededAfter(
		fmt.Errorf("gateway is in a transitional state, cannot be modified or deleted"),
		15*time.Second,
	)
	requeueGateTargetDeleting = ackrequeue.NeededAfter(
		fmt.Errorf("Gateway still has targets, requeuing"),
		10*time.Second,
	)
)

// gatewaySettled returns true when the gateway is not in a transitional state
// (CREATING, UPDATING, DELETING). This allows updates from READY, FAILED, and
// UPDATE_UNSUCCESSFUL states so the user can attempt to fix issues.
func gatewaySettled(r *resource) bool {
	if r.ko.Status.Status == nil {
		return false
	}
	switch svcsdktypes.GatewayStatus(*r.ko.Status.Status) {
	case svcsdktypes.GatewayStatusCreating,
		svcsdktypes.GatewayStatusUpdating,
		svcsdktypes.GatewayStatusDeleting:
		return false
	default:
		return true
	}
}

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

	return syncTags(
		ctx,
		rm.sdkapi,
		rm.metrics,
		resourceARN,
		desiredTags,
		existingTags,
	)
}
