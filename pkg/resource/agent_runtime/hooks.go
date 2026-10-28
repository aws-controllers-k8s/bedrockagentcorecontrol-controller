package agent_runtime

import (
	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"

	"github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/pkg/tags"
)

var syncTags = tags.SyncTags

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
