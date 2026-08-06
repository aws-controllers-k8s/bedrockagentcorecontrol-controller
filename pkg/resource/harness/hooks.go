package harness

import (
	"context"

	svcapitypes "github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/apis/v1alpha1"
	"github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/pkg/tags"
	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackmetrics "github.com/aws-controllers-k8s/runtime/pkg/metrics"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"
	svcsdktypes "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol/types"
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

func prepareGetHarnessInput(input *svcsdk.GetHarnessInput) {
	input.HarnessVersion = nil
}

func setAgentRuntimeStatus(
	ko *svcapitypes.Harness,
	environment svcsdktypes.HarnessEnvironmentProvider,
) {
	ko.Status.AgentRuntimeARN = nil
	ko.Status.AgentRuntimeID = nil
	ko.Status.AgentRuntimeName = nil

	runtimeEnvironment, ok := environment.(*svcsdktypes.HarnessEnvironmentProviderMemberAgentCoreRuntimeEnvironment)
	if !ok || runtimeEnvironment == nil {
		return
	}
	ko.Status.AgentRuntimeARN = runtimeEnvironment.Value.AgentRuntimeArn
	ko.Status.AgentRuntimeID = runtimeEnvironment.Value.AgentRuntimeId
	ko.Status.AgentRuntimeName = runtimeEnvironment.Value.AgentRuntimeName
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

func setUpdateWrapperFields(
	input *svcsdk.UpdateHarnessInput,
	createInput *svcsdk.CreateHarnessInput,
	delta *ackcompare.Delta,
) {
	if delta.DifferentAt("Spec.AuthorizerConfiguration") {
		input.AuthorizerConfiguration = &svcsdktypes.UpdatedAuthorizerConfiguration{
			OptionalValue: createInput.AuthorizerConfiguration,
		}
	}
	if delta.DifferentAt("Spec.EnvironmentArtifact") {
		input.EnvironmentArtifact = &svcsdktypes.UpdatedHarnessEnvironmentArtifact{
			OptionalValue: createInput.EnvironmentArtifact,
		}
	}
	if delta.DifferentAt("Spec.Memory") {
		input.Memory = &svcsdktypes.UpdatedHarnessMemoryConfiguration{
			OptionalValue: createInput.Memory,
		}
	}
}
