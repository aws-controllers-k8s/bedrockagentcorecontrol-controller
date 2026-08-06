package harness_endpoint

import (
	"context"
	"strings"
	"testing"

	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackmetrics "github.com/aws-controllers-k8s/runtime/pkg/metrics"
	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"

	svcapitypes "github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/apis/v1alpha1"
)

func TestIsSyncedByHarnessEndpointStatus(t *testing.T) {
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
				&resource{ko: &svcapitypes.HarnessEndpoint{
					Status: svcapitypes.HarnessEndpointStatus{Status: tt.status},
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

func TestTargetVersionFromRead(t *testing.T) {
	desired := aws.String("1")
	live := aws.String("2")

	if got := targetVersionFromRead(desired, live); got == nil || *got != "2" {
		t.Fatalf("targetVersionFromRead with live version = %v, want 2", got)
	}
	if got := targetVersionFromRead(desired, nil); got != desired {
		t.Fatalf("targetVersionFromRead without live version = %v, want desired", got)
	}
}

func TestSDKUpdateSkipsNoOp(t *testing.T) {
	desired := &resource{ko: &svcapitypes.HarnessEndpoint{}}
	latest := &resource{ko: &svcapitypes.HarnessEndpoint{}}

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
	desired := &resource{ko: &svcapitypes.HarnessEndpoint{}}
	latest := &resource{ko: &svcapitypes.HarnessEndpoint{
		Status: svcapitypes.HarnessEndpointStatus{Status: aws.String("UPDATING")},
	}}
	delta := ackcompare.NewDelta()
	delta.Add("Spec.TargetVersion", aws.String("1"), aws.String("2"))

	got, err := (&resourceManager{}).sdkUpdate(
		context.Background(), desired, latest, delta,
	)
	if got != nil {
		t.Error("sdkUpdate returned a resource while the endpoint was transitional")
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
		if resourceARN != "arn:aws:bedrock-agentcore:us-west-2:123456789012:harness-endpoint/test" {
			t.Errorf("resource ARN = %q", resourceARN)
		}
		if desiredTags["phase"] != "new" || existingTags["phase"] != "old" {
			t.Errorf("unexpected tag maps: desired=%v existing=%v", desiredTags, existingTags)
		}
		return nil
	}

	resourceARN := ackv1alpha1.AWSResourceName(
		"arn:aws:bedrock-agentcore:us-west-2:123456789012:harness-endpoint/test",
	)
	desired := &resource{ko: &svcapitypes.HarnessEndpoint{
		Spec: svcapitypes.HarnessEndpointSpec{
			Tags: map[string]*string{"phase": aws.String("new")},
		},
	}}
	latest := &resource{ko: &svcapitypes.HarnessEndpoint{
		Spec: svcapitypes.HarnessEndpointSpec{
			Tags: map[string]*string{"phase": aws.String("old")},
		},
		Status: svcapitypes.HarnessEndpointStatus{
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
