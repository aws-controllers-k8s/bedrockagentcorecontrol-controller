package harness

import (
	"context"
	"strings"
	"testing"

	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackmetrics "github.com/aws-controllers-k8s/runtime/pkg/metrics"
	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"
	svcsdktypes "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol/types"

	svcapitypes "github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/apis/v1alpha1"
)

func TestSetUpdateWrapperFieldsUnchanged(t *testing.T) {
	input := &svcsdk.UpdateHarnessInput{}
	createInput := &svcsdk.CreateHarnessInput{
		AuthorizerConfiguration: &svcsdktypes.AuthorizerConfigurationMemberCustomJWTAuthorizer{},
		EnvironmentArtifact:     &svcsdktypes.HarnessEnvironmentArtifactMemberContainerConfiguration{},
		Memory:                  &svcsdktypes.HarnessMemoryConfigurationMemberDisabled{},
	}

	setUpdateWrapperFields(input, createInput, ackcompare.NewDelta())

	if input.AuthorizerConfiguration != nil {
		t.Error("authorizer wrapper was set without a delta")
	}
	if input.EnvironmentArtifact != nil {
		t.Error("environment artifact wrapper was set without a delta")
	}
	if input.Memory != nil {
		t.Error("memory wrapper was set without a delta")
	}
}

func TestPrepareGetHarnessInputReadsCurrentVersion(t *testing.T) {
	input := &svcsdk.GetHarnessInput{
		HarnessId:      aws.String("harness-1234567890"),
		HarnessVersion: aws.String("7"),
	}

	prepareGetHarnessInput(input)

	if input.HarnessVersion != nil {
		t.Fatalf("HarnessVersion = %q, want nil", *input.HarnessVersion)
	}
	if aws.ToString(input.HarnessId) != "harness-1234567890" {
		t.Errorf("HarnessId was changed: %q", aws.ToString(input.HarnessId))
	}
}

func TestSetAgentRuntimeStatus(t *testing.T) {
	ko := &svcapitypes.Harness{}
	environment := &svcsdktypes.HarnessEnvironmentProviderMemberAgentCoreRuntimeEnvironment{
		Value: svcsdktypes.HarnessAgentCoreRuntimeEnvironment{
			AgentRuntimeArn:  aws.String("arn:aws:bedrock-agentcore:us-west-2:123456789012:runtime/test"),
			AgentRuntimeId:   aws.String("runtime-1234567890"),
			AgentRuntimeName: aws.String("managed-runtime"),
		},
	}

	setAgentRuntimeStatus(ko, environment)

	if aws.ToString(ko.Status.AgentRuntimeARN) != aws.ToString(environment.Value.AgentRuntimeArn) {
		t.Errorf("AgentRuntimeARN was not copied")
	}
	if aws.ToString(ko.Status.AgentRuntimeID) != aws.ToString(environment.Value.AgentRuntimeId) {
		t.Errorf("AgentRuntimeID was not copied")
	}
	if aws.ToString(ko.Status.AgentRuntimeName) != aws.ToString(environment.Value.AgentRuntimeName) {
		t.Errorf("AgentRuntimeName was not copied")
	}

	setAgentRuntimeStatus(ko, nil)
	if ko.Status.AgentRuntimeARN != nil ||
		ko.Status.AgentRuntimeID != nil ||
		ko.Status.AgentRuntimeName != nil {
		t.Error("AgentRuntime status was not cleared when the environment disappeared")
	}
}

func TestSetUpdateWrapperFieldsSet(t *testing.T) {
	authorizer := &svcsdktypes.AuthorizerConfigurationMemberCustomJWTAuthorizer{}
	artifact := &svcsdktypes.HarnessEnvironmentArtifactMemberContainerConfiguration{}
	memory := &svcsdktypes.HarnessMemoryConfigurationMemberDisabled{}
	createInput := &svcsdk.CreateHarnessInput{
		AuthorizerConfiguration: authorizer,
		EnvironmentArtifact:     artifact,
		Memory:                  memory,
	}
	input := &svcsdk.UpdateHarnessInput{}
	delta := ackcompare.NewDelta()
	delta.Add("Spec.AuthorizerConfiguration", nil, struct{}{})
	delta.Add("Spec.EnvironmentArtifact", nil, struct{}{})
	delta.Add("Spec.Memory", nil, struct{}{})

	setUpdateWrapperFields(input, createInput, delta)

	if input.AuthorizerConfiguration == nil ||
		input.AuthorizerConfiguration.OptionalValue != authorizer {
		t.Error("authorizer wrapper did not contain the converted value")
	}
	if input.EnvironmentArtifact == nil ||
		input.EnvironmentArtifact.OptionalValue != artifact {
		t.Error("environment artifact wrapper did not contain the converted value")
	}
	if input.Memory == nil || input.Memory.OptionalValue != memory {
		t.Error("memory wrapper did not contain the converted value")
	}
}

func TestSetUpdateWrapperFieldsClear(t *testing.T) {
	input := &svcsdk.UpdateHarnessInput{}
	delta := ackcompare.NewDelta()
	delta.Add("Spec.AuthorizerConfiguration", struct{}{}, nil)
	delta.Add("Spec.EnvironmentArtifact", struct{}{}, nil)
	delta.Add("Spec.Memory", struct{}{}, nil)

	setUpdateWrapperFields(input, &svcsdk.CreateHarnessInput{}, delta)

	if input.AuthorizerConfiguration == nil ||
		input.AuthorizerConfiguration.OptionalValue != nil {
		t.Error("authorizer clear did not produce an empty wrapper")
	}
	if input.EnvironmentArtifact == nil ||
		input.EnvironmentArtifact.OptionalValue != nil {
		t.Error("environment artifact clear did not produce an empty wrapper")
	}
	if input.Memory == nil || input.Memory.OptionalValue != nil {
		t.Error("memory clear did not produce an empty wrapper")
	}
}

func TestIsSyncedByHarnessStatus(t *testing.T) {
	tests := []struct {
		name   string
		status *string
		want   bool
	}{
		{name: "missing", status: nil, want: false},
		{name: "creating", status: aws.String("CREATING"), want: false},
		{name: "updating", status: aws.String("UPDATING"), want: false},
		{name: "ready", status: aws.String("READY"), want: true},
		{name: "create failed", status: aws.String("CREATE_FAILED"), want: true},
		{name: "update failed", status: aws.String("UPDATE_FAILED"), want: true},
		{name: "delete failed", status: aws.String("DELETE_FAILED"), want: true},
	}

	rm := &resourceManager{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rm.IsSynced(
				context.Background(),
				&resource{ko: &svcapitypes.Harness{
					Status: svcapitypes.HarnessStatus{Status: tt.status},
				}},
			)
			if err != nil {
				t.Fatalf("IsSynced returned an error: %v", err)
			}
			if got != tt.want {
				t.Errorf("IsSynced = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSDKUpdateSkipsNoOp(t *testing.T) {
	desired := &resource{ko: &svcapitypes.Harness{}}
	latest := &resource{ko: &svcapitypes.Harness{}}

	got, err := (&resourceManager{}).sdkUpdate(
		context.Background(), desired, latest, ackcompare.NewDelta(),
	)
	if err != nil {
		t.Fatalf("sdkUpdate returned an error: %v", err)
	}
	if got != desired {
		t.Error("sdkUpdate did not return the desired resource for a no-op")
	}
}

func TestSDKUpdateRequeuesWhileTransitional(t *testing.T) {
	desired := &resource{ko: &svcapitypes.Harness{}}
	latest := &resource{ko: &svcapitypes.Harness{
		Status: svcapitypes.HarnessStatus{Status: aws.String("UPDATING")},
	}}
	delta := ackcompare.NewDelta()
	delta.Add("Spec.MaxIterations", aws.Int64(1), aws.Int64(2))

	got, err := (&resourceManager{}).sdkUpdate(
		context.Background(), desired, latest, delta,
	)
	if got != nil {
		t.Error("sdkUpdate returned a resource while the Harness was transitional")
	}
	if err == nil || !strings.Contains(err.Error(), "cannot be updated") {
		t.Fatalf("sdkUpdate error = %v, want a transitional-state requeue", err)
	}
}

func TestSDKUpdateSynchronizesTagOnlyDelta(t *testing.T) {
	originalSyncTags := syncTags
	defer func() { syncTags = originalSyncTags }()

	var called bool
	syncTags = func(
		_ context.Context,
		_ *svcsdk.Client,
		_ *ackmetrics.Metrics,
		resourceARN string,
		desiredTags map[string]string,
		existingTags map[string]string,
	) error {
		called = true
		if resourceARN != "arn:aws:bedrock-agentcore:us-west-2:123456789012:harness/test" {
			t.Errorf("resource ARN = %q", resourceARN)
		}
		if desiredTags["phase"] != "new" || existingTags["phase"] != "old" {
			t.Errorf("unexpected tag maps: desired=%v existing=%v", desiredTags, existingTags)
		}
		return nil
	}

	resourceARN := ackv1alpha1.AWSResourceName(
		"arn:aws:bedrock-agentcore:us-west-2:123456789012:harness/test",
	)
	desired := &resource{ko: &svcapitypes.Harness{
		Spec: svcapitypes.HarnessSpec{
			Tags: map[string]*string{"phase": aws.String("new")},
		},
	}}
	latest := &resource{ko: &svcapitypes.Harness{
		Spec: svcapitypes.HarnessSpec{
			Tags: map[string]*string{"phase": aws.String("old")},
		},
		Status: svcapitypes.HarnessStatus{
			ACKResourceMetadata: &ackv1alpha1.ResourceMetadata{ARN: &resourceARN},
		},
	}}
	delta := ackcompare.NewDelta()
	delta.Add("Spec.Tags", latest.ko.Spec.Tags, desired.ko.Spec.Tags)

	got, err := (&resourceManager{}).sdkUpdate(
		context.Background(), desired, latest, delta,
	)
	if err != nil {
		t.Fatalf("sdkUpdate returned an error: %v", err)
	}
	if got != desired {
		t.Error("sdkUpdate did not return the desired resource for a tag-only update")
	}
	if !called {
		t.Error("sdkUpdate did not synchronize tags")
	}
}
