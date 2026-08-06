package harness_endpoint

import (
	"context"

	"github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/pkg/tags"
	ackmetrics "github.com/aws-controllers-k8s/runtime/pkg/metrics"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"
)

var syncTags = func(
	ctx context.Context,
	client *svcsdk.Client,
	metrics *ackmetrics.Metrics,
	resourceARN string,
	desiredTags map[string]string,
	existingTags map[string]string,
) error {
	return tags.SyncTags(
		ctx, client, metrics, resourceARN, desiredTags, existingTags,
	)
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

// targetVersionFromRead returns the endpoint version that should be compared
// with Spec.TargetVersion after GetHarnessEndpoint. The service does not return
// TargetVersion in its response, but LiveVersion is the version the endpoint is
// actually serving once it is ready. Preserve the desired value while AWS has
// not reported a live version yet so transitional reads do not create a false
// delta.
func targetVersionFromRead(desired, live *string) *string {
	if live != nil {
		return live
	}
	return desired
}
