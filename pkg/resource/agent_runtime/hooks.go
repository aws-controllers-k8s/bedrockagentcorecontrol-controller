package agent_runtime

import (
	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"

	svcapitypes "github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/apis/v1alpha1"
	"github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/pkg/filesystemconfig"
	"github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/pkg/tags"
)

var syncTags = tags.SyncTags

// normalizeFilesystemConfigurations is called from the ReadOne
// sdk_read_one_post_set_output hook. ReadOne is the only operation whose
// output rebuilds Spec.FilesystemConfigurations. See filesystemconfig.Normalize.
func normalizeFilesystemConfigurations(ko, declared *svcapitypes.AgentRuntime) {
	ko.Spec.FilesystemConfigurations = filesystemconfig.Normalize(
		ko.Spec.FilesystemConfigurations, declared.Spec.FilesystemConfigurations,
	)
}

// customPostCompare treats omitted platform and capacity provider settings as
// unmanaged so that a spec written before either field existed can reconcile
// without creating a new runtime version.
func customPostCompare(delta *ackcompare.Delta, desired, _ *resource) {
	ignorePlatformVersion := desired.ko.Spec.PlatformVersion == nil
	ignoreCapacityProvider := desired.ko.Spec.CapacityProviderConfiguration == nil
	kept := delta.Differences[:0]
	for _, difference := range delta.Differences {
		if (ignorePlatformVersion && difference.Path.Contains("Spec.PlatformVersion")) ||
			(ignoreCapacityProvider && difference.Path.Contains("Spec.CapacityProviderConfiguration")) {
			continue
		}
		kept = append(kept, difference)
	}
	delta.Differences = kept
}
