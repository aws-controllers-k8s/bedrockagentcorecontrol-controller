package agent_runtime

import (
	"fmt"
	"time"

	"github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/pkg/tags"
	ackrequeue "github.com/aws-controllers-k8s/runtime/pkg/requeue"
	svcsdktypes "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol/types"
)

var (
	requeueNotReady = ackrequeue.NeededAfter(
		fmt.Errorf("agentruntime is not in Ready state, cannot be modified or deleted"),
		15*time.Second,
	)
)

func agentRuntimeReady(rt *resource) bool {
	return rt.ko.Status.Status != nil && *rt.ko.Status.Status == string(svcsdktypes.AgentRuntimeEndpointStatusReady)
}

var syncTags = tags.SyncTags
