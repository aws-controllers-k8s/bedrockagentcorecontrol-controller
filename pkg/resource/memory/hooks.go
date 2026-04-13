package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/pkg/tags"
	ackrequeue "github.com/aws-controllers-k8s/runtime/pkg/requeue"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"
	svcsdktypes "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol/types"
)

var syncTags = tags.SyncTags

var (
	requeueNotReady = ackrequeue.NeededAfter(
		fmt.Errorf("memory is in a transitional state, cannot be modified or deleted"),
		15*time.Second,
	)
)

func memorySettled(r *resource) bool {
	if r.ko.Status.Status == nil {
		return false
	}
	switch svcsdktypes.MemoryStatus(*r.ko.Status.Status) {
	case svcsdktypes.MemoryStatusCreating,
		svcsdktypes.MemoryStatusDeleting:
		return false
	default:
		return true
	}
}

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

func (rm *resourceManager) syncTags(
	ctx context.Context,
	desired *resource,
	latest *resource,
) error {
	resourceARN := string(*latest.ko.Status.ACKResourceMetadata.ARN)
	desiredTags := aws.ToStringMap(desired.ko.Spec.Tags)
	existingTags := aws.ToStringMap(latest.ko.Spec.Tags)
	return syncTags(
		ctx, rm.sdkapi, rm.metrics,
		resourceARN, desiredTags, existingTags,
	)
}
