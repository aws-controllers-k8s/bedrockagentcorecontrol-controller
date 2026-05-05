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

package gateway_target

import (
	"testing"

	svcapitypes "github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/apis/v1alpha1"
	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	"github.com/aws/aws-sdk-go-v2/aws"
)

// makeResource builds a resource with the given ToolDefinitions wired into
// Spec.TargetConfiguration.Mcp.Lambda.ToolSchema.InlinePayload.
func makeResource(toolDefs []*svcapitypes.ToolDefinition) *resource {
	return &resource{
		ko: &svcapitypes.GatewayTarget{
			Spec: svcapitypes.GatewayTargetSpec{
				TargetConfiguration: &svcapitypes.TargetConfiguration{
					Mcp: &svcapitypes.McpTargetConfiguration{
						Lambda: &svcapitypes.McpLambdaTargetConfiguration{
							ToolSchema: &svcapitypes.ToolSchema{
								InlinePayload: toolDefs,
							},
						},
					},
				},
			},
		},
	}
}

func TestCompareInlinePayloadToolDefinitions_BothNil(t *testing.T) {
	a := &resource{ko: &svcapitypes.GatewayTarget{}}
	b := &resource{ko: &svcapitypes.GatewayTarget{}}
	delta := ackcompare.NewDelta()

	compareInlinePayloadToolDefinitions(delta, a, b)

	if len(delta.Differences) != 0 {
		t.Errorf("expected no differences when both are nil, got %d", len(delta.Differences))
	}
}

func TestCompareInlinePayloadToolDefinitions_Identical(t *testing.T) {
	defs := func() []*svcapitypes.ToolDefinition {
		return []*svcapitypes.ToolDefinition{
			{
				Name:        aws.String("myTool"),
				Description: aws.String("does stuff"),
				InputSchema: aws.String(`{"type":"object","properties":{"param1":{"type":"string"}},"required":["param1"]}`),
			},
		}
	}
	a := makeResource(defs())
	b := makeResource(defs())
	delta := ackcompare.NewDelta()

	compareInlinePayloadToolDefinitions(delta, a, b)

	if len(delta.Differences) != 0 {
		t.Errorf("expected no differences for identical tools, got %d", len(delta.Differences))
	}
}

func TestCompareInlinePayloadToolDefinitions_NullFieldsIgnored(t *testing.T) {
	// Simulates the user spec (no null fields) vs the API response (explicit nulls).
	userSchema := aws.String(`{"type":"object","properties":{"param1":{"type":"string"}},"required":["param1"]}`)
	apiSchema := aws.String(`{"type":"object","description":null,"items":null,"properties":{"param1":{"type":"string","description":null,"items":null,"properties":null,"required":null}},"required":["param1"]}`)

	a := makeResource([]*svcapitypes.ToolDefinition{
		{Name: aws.String("myTool"), Description: aws.String("does stuff"), InputSchema: userSchema},
	})
	b := makeResource([]*svcapitypes.ToolDefinition{
		{Name: aws.String("myTool"), Description: aws.String("does stuff"), InputSchema: apiSchema},
	})
	delta := ackcompare.NewDelta()

	compareInlinePayloadToolDefinitions(delta, a, b)

	if len(delta.Differences) != 0 {
		t.Errorf("expected no differences when only null fields differ, got %d", len(delta.Differences))
	}
}

func TestCompareInlinePayloadToolDefinitions_KeyCasingIgnored(t *testing.T) {
	// User sends lowercase keys, API returns uppercase keys.
	userSchema := aws.String(`{"type":"object","properties":{"param1":{"type":"string"}},"required":["param1"]}`)
	apiSchema := aws.String(`{"Type":"object","Properties":{"param1":{"Type":"string"}},"Required":["param1"]}`)

	a := makeResource([]*svcapitypes.ToolDefinition{
		{Name: aws.String("myTool"), Description: aws.String("desc"), InputSchema: userSchema},
	})
	b := makeResource([]*svcapitypes.ToolDefinition{
		{Name: aws.String("myTool"), Description: aws.String("desc"), InputSchema: apiSchema},
	})
	delta := ackcompare.NewDelta()

	compareInlinePayloadToolDefinitions(delta, a, b)

	if len(delta.Differences) != 0 {
		t.Errorf("expected no differences when only key casing differs, got %d", len(delta.Differences))
	}
}

func TestCompareInlinePayloadToolDefinitions_CasingAndNullsCombined(t *testing.T) {
	// Both issues at once: uppercase keys AND explicit nulls from the API.
	userSchema := aws.String(`{"type":"object","properties":{"param1":{"type":"string"}},"required":["param1"]}`)
	apiSchema := aws.String(`{"Type":"object","Description":null,"Items":null,"Properties":{"param1":{"Type":"string","Description":null,"Items":null,"Properties":null,"Required":null}},"Required":["param1"]}`)

	a := makeResource([]*svcapitypes.ToolDefinition{
		{Name: aws.String("myTool"), Description: aws.String("desc"), InputSchema: userSchema},
	})
	b := makeResource([]*svcapitypes.ToolDefinition{
		{Name: aws.String("myTool"), Description: aws.String("desc"), InputSchema: apiSchema},
	})
	delta := ackcompare.NewDelta()

	compareInlinePayloadToolDefinitions(delta, a, b)

	if len(delta.Differences) != 0 {
		t.Errorf("expected no differences with combined casing + null diffs, got %d", len(delta.Differences))
	}
}

func TestCompareInlinePayloadToolDefinitions_DifferentOrder(t *testing.T) {
	// Tools in different order should still match by name.
	a := makeResource([]*svcapitypes.ToolDefinition{
		{Name: aws.String("toolA"), Description: aws.String("a")},
		{Name: aws.String("toolB"), Description: aws.String("b")},
	})
	b := makeResource([]*svcapitypes.ToolDefinition{
		{Name: aws.String("toolB"), Description: aws.String("b")},
		{Name: aws.String("toolA"), Description: aws.String("a")},
	})
	delta := ackcompare.NewDelta()

	compareInlinePayloadToolDefinitions(delta, a, b)

	if len(delta.Differences) != 0 {
		t.Errorf("expected no differences when tools are reordered, got %d", len(delta.Differences))
	}
}

func TestCompareInlinePayloadToolDefinitions_DifferentDescription(t *testing.T) {
	a := makeResource([]*svcapitypes.ToolDefinition{
		{Name: aws.String("myTool"), Description: aws.String("old desc")},
	})
	b := makeResource([]*svcapitypes.ToolDefinition{
		{Name: aws.String("myTool"), Description: aws.String("new desc")},
	})
	delta := ackcompare.NewDelta()

	compareInlinePayloadToolDefinitions(delta, a, b)

	if len(delta.Differences) != 1 {
		t.Errorf("expected 1 difference for changed description, got %d", len(delta.Differences))
	}
}

func TestCompareInlinePayloadToolDefinitions_DifferentSchemaValue(t *testing.T) {
	a := makeResource([]*svcapitypes.ToolDefinition{
		{Name: aws.String("myTool"), InputSchema: aws.String(`{"type":"object"}`)},
	})
	b := makeResource([]*svcapitypes.ToolDefinition{
		{Name: aws.String("myTool"), InputSchema: aws.String(`{"type":"string"}`)},
	})
	delta := ackcompare.NewDelta()

	compareInlinePayloadToolDefinitions(delta, a, b)

	if len(delta.Differences) != 1 {
		t.Errorf("expected 1 difference for changed schema value, got %d", len(delta.Differences))
	}
}

func TestCompareInlinePayloadToolDefinitions_LengthMismatch(t *testing.T) {
	a := makeResource([]*svcapitypes.ToolDefinition{
		{Name: aws.String("toolA")},
		{Name: aws.String("toolB")},
	})
	b := makeResource([]*svcapitypes.ToolDefinition{
		{Name: aws.String("toolA")},
	})
	delta := ackcompare.NewDelta()

	compareInlinePayloadToolDefinitions(delta, a, b)

	if len(delta.Differences) != 1 {
		t.Errorf("expected 1 difference for length mismatch, got %d", len(delta.Differences))
	}
}

func TestCompareInlinePayloadToolDefinitions_MissingTool(t *testing.T) {
	a := makeResource([]*svcapitypes.ToolDefinition{
		{Name: aws.String("toolA")},
	})
	b := makeResource([]*svcapitypes.ToolDefinition{
		{Name: aws.String("toolB")},
	})
	delta := ackcompare.NewDelta()

	compareInlinePayloadToolDefinitions(delta, a, b)

	if len(delta.Differences) != 1 {
		t.Errorf("expected 1 difference when tool name not found, got %d", len(delta.Differences))
	}
}

func TestCompareInlinePayloadToolDefinitions_OutputSchemaComparison(t *testing.T) {
	userSchema := aws.String(`{"type":"object"}`)
	apiSchema := aws.String(`{"Type":"object","Description":null}`)

	a := makeResource([]*svcapitypes.ToolDefinition{
		{Name: aws.String("myTool"), OutputSchema: userSchema},
	})
	b := makeResource([]*svcapitypes.ToolDefinition{
		{Name: aws.String("myTool"), OutputSchema: apiSchema},
	})
	delta := ackcompare.NewDelta()

	compareInlinePayloadToolDefinitions(delta, a, b)

	if len(delta.Differences) != 0 {
		t.Errorf("expected no differences for OutputSchema with casing + null diffs, got %d", len(delta.Differences))
	}
}

func TestCompareInlinePayloadToolDefinitions_OneNilSchema(t *testing.T) {
	// One side has InputSchema, the other doesn't — should be a difference.
	a := makeResource([]*svcapitypes.ToolDefinition{
		{Name: aws.String("myTool"), InputSchema: aws.String(`{"type":"object"}`)},
	})
	b := makeResource([]*svcapitypes.ToolDefinition{
		{Name: aws.String("myTool")},
	})
	delta := ackcompare.NewDelta()

	compareInlinePayloadToolDefinitions(delta, a, b)

	if len(delta.Differences) != 1 {
		t.Errorf("expected 1 difference when one side has nil InputSchema, got %d", len(delta.Differences))
	}
}
