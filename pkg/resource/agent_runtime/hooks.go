package agent_runtime

import (
	"context"
	"fmt"

	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"

	svcapitypes "github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/apis/v1alpha1"
	"github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/pkg/filesystemconfig"
	"github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/pkg/tags"
)

var syncTags = tags.SyncTags

// GetAgentRuntime does not return tags. Never leave the copied desired tags in
// the observed resource: that hides every tag delta from sdkUpdate.
func (rm *resourceManager) readRuntimeTags(ctx context.Context, ko *svcapitypes.AgentRuntime) error {
	if ko.Status.ACKResourceMetadata == nil || ko.Status.ACKResourceMetadata.ARN == nil {
		return fmt.Errorf("cannot read runtime tags without an observed ARN")
	}
	arn := string(*ko.Status.ACKResourceMetadata.ARN)
	output, err := rm.sdkapi.ListTagsForResource(ctx, &svcsdk.ListTagsForResourceInput{ResourceArn: &arn})
	rm.metrics.RecordAPICall("GET", "ListTagsForResource", err)
	if err != nil {
		return err
	}
	ko.Spec.Tags = aws.StringMap(output.Tags)
	return nil
}

// normalizeFilesystemConfigurations is called from the ReadOne
// sdk_read_one_post_set_output hook. ReadOne is the only operation whose
// output rebuilds Spec.FilesystemConfigurations. See filesystemconfig.Normalize.
func normalizeFilesystemConfigurations(ko, declared *svcapitypes.AgentRuntime) {
	ko.Spec.FilesystemConfigurations = filesystemconfig.Normalize(
		ko.Spec.FilesystemConfigurations, declared.Spec.FilesystemConfigurations,
	)
}

// customPostCompare drops the Spec.PlatformVersion difference when the spec
// leaves the field unset. The service defaults the platform version on create
// and keeps the current one when UpdateAgentRuntime omits it, so unset carries
// no intent. Without this, a resource whose spec predates the field is updated
// on its first reconcile after an upgrade, because late initialization only
// runs after the update.
func customPostCompare(delta *ackcompare.Delta, a, b *resource) {
	if a.ko.Spec.PlatformVersion != nil {
		return
	}
	kept := delta.Differences[:0]
	for _, d := range delta.Differences {
		if !d.Path.Contains("Spec.PlatformVersion") {
			kept = append(kept, d)
		}
	}
	delta.Differences = kept
}
