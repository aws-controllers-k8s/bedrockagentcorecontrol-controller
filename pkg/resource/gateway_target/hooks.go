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
	"encoding/json"
	"fmt"
	"strings"

	svcapitypes "github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/apis/v1alpha1"
	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"
	svcsdktypes "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol/types"
)

// stringToSchemaDefinition unmarshals a JSON string into an SDK
// SchemaDefinition. The SchemaDefinition type is recursive (it contains
// Items *SchemaDefinition and Properties map[string]SchemaDefinition) which
// cannot be directly represented in a CRD. We accept it as a JSON string
// from the user and unmarshal it here.
func stringToSchemaDefinition(s *string) (*svcsdktypes.SchemaDefinition, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	var sd svcsdktypes.SchemaDefinition
	if err := json.Unmarshal([]byte(*s), &sd); err != nil {
		return nil, fmt.Errorf("failed to unmarshal SchemaDefinition JSON: %w", err)
	}
	return &sd, nil
}

// getMcpLambdaInlinePayload returns the inline payload ToolDefinitions from
// the TargetConfiguration if the target is an MCP Lambda target with inline
// payload tool schema. Returns nil if the path doesn't apply.
func getMcpLambdaInlinePayload(tc *svcapitypes.TargetConfiguration) []*svcapitypes.ToolDefinition {
	if tc == nil || tc.Mcp == nil || tc.Mcp.Lambda == nil ||
		tc.Mcp.Lambda.ToolSchema == nil || tc.Mcp.Lambda.ToolSchema.InlinePayload == nil {
		return nil
	}
	return tc.Mcp.Lambda.ToolSchema.InlinePayload
}

// getSDKInlinePayload extracts the InlinePayload ToolDefinition slice from an
// SDK TargetConfiguration, returning nil if the path doesn't match.
func getSDKInlinePayload(tc svcsdktypes.TargetConfiguration) []svcsdktypes.ToolDefinition {
	mcpMember, ok := tc.(*svcsdktypes.TargetConfigurationMemberMcp)
	if !ok || mcpMember == nil {
		return nil
	}
	lambdaMember, ok := mcpMember.Value.(*svcsdktypes.McpTargetConfigurationMemberLambda)
	if !ok || lambdaMember == nil {
		return nil
	}
	inlinePayloadMember, ok := lambdaMember.Value.ToolSchema.(*svcsdktypes.ToolSchemaMemberInlinePayload)
	if !ok || inlinePayloadMember == nil {
		return nil
	}
	return inlinePayloadMember.Value
}

// setSchemaDefinitionsOnInput walks the SDK input's ToolDefinition slice and
// sets InputSchema/OutputSchema by unmarshaling the JSON strings from the CR.
func setSchemaDefinitionsOnInput(
	toolDefs []*svcapitypes.ToolDefinition,
	sdkToolDefs []svcsdktypes.ToolDefinition,
) error {
	for i, td := range toolDefs {
		if i >= len(sdkToolDefs) {
			break
		}
		if td.InputSchema != nil {
			sd, err := stringToSchemaDefinition(td.InputSchema)
			if err != nil {
				return err
			}
			sdkToolDefs[i].InputSchema = sd
		}
		if td.OutputSchema != nil {
			sd, err := stringToSchemaDefinition(td.OutputSchema)
			if err != nil {
				return err
			}
			sdkToolDefs[i].OutputSchema = sd
		}
	}
	return nil
}

// setSchemaDefinitionsOnCreateInput unmarshals the InputSchema and OutputSchema
// JSON strings from the custom resource into SDK SchemaDefinition objects on
// the CreateGatewayTarget input.
func setSchemaDefinitionsOnCreateInput(desired *resource, input *svcsdk.CreateGatewayTargetInput) error {
	toolDefs := getMcpLambdaInlinePayload(desired.ko.Spec.TargetConfiguration)
	if toolDefs == nil {
		return nil
	}
	sdkToolDefs := getSDKInlinePayload(input.TargetConfiguration)
	if sdkToolDefs == nil {
		return nil
	}
	return setSchemaDefinitionsOnInput(toolDefs, sdkToolDefs)
}

// setSchemaDefinitionsOnUpdateInput unmarshals the InputSchema and OutputSchema
// JSON strings from the custom resource into SDK SchemaDefinition objects on
// the UpdateGatewayTarget input.
func setSchemaDefinitionsOnUpdateInput(desired *resource, input *svcsdk.UpdateGatewayTargetInput) error {
	toolDefs := getMcpLambdaInlinePayload(desired.ko.Spec.TargetConfiguration)
	if toolDefs == nil {
		return nil
	}
	sdkToolDefs := getSDKInlinePayload(input.TargetConfiguration)
	if sdkToolDefs == nil {
		return nil
	}
	return setSchemaDefinitionsOnInput(toolDefs, sdkToolDefs)
}

// schemaDefinitionToString marshals an SDK SchemaDefinition into a JSON string.
// This is the inverse of stringToSchemaDefinition and is used when reading
// the resource back from AWS.
func schemaDefinitionToString(sd *svcsdktypes.SchemaDefinition) (*string, error) {
	if sd == nil {
		return nil, nil
	}
	b, err := json.Marshal(sd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal SchemaDefinition to JSON: %w", err)
	}
	s := string(b)
	return &s, nil
}

// setSchemaDefinitionsFromSDKResponse reads the SDK GetGatewayTarget response
// and populates the InputSchema and OutputSchema JSON strings on the custom
// resource's ToolDefinition fields from the SDK SchemaDefinition objects.
func setSchemaDefinitionsFromSDKResponse(
	ko *svcapitypes.GatewayTarget,
	resp *svcsdk.GetGatewayTargetOutput,
) error {
	if resp.TargetConfiguration == nil {
		return nil
	}
	sdkToolDefs := getSDKInlinePayload(resp.TargetConfiguration)
	if sdkToolDefs == nil {
		return nil
	}
	crToolDefs := getMcpLambdaInlinePayload(ko.Spec.TargetConfiguration)
	if crToolDefs == nil {
		return nil
	}
	for i, sdkTD := range sdkToolDefs {
		if i >= len(crToolDefs) {
			break
		}
		inputStr, err := schemaDefinitionToString(sdkTD.InputSchema)
		if err != nil {
			return err
		}
		crToolDefs[i].InputSchema = inputStr

		outputStr, err := schemaDefinitionToString(sdkTD.OutputSchema)
		if err != nil {
			return err
		}
		crToolDefs[i].OutputSchema = outputStr
	}
	return nil
}

// normalizeJSON takes a JSON string, unmarshals it, and re-marshals it to
// produce a canonical representation. This eliminates differences caused by
// key ordering, whitespace variations, and explicit null fields that the AWS
// API may add for unset values.
func normalizeJSON(s *string) string {
	if s == nil || *s == "" {
		return ""
	}
	var v any
	if err := json.Unmarshal([]byte(*s), &v); err != nil {
		// If it's not valid JSON, return the raw string so the comparison
		// still detects a difference.
		return *s
	}
	v = canonicalizeJSON(v)
	b, err := json.Marshal(v)
	if err != nil {
		return *s
	}
	return string(b)
}

// canonicalizeJSON recursively normalizes an unmarshaled JSON value by removing
// map entries whose value is nil and lowercasing all map keys. This ensures
// that explicit null fields returned by the AWS API (e.g. "Description":null)
// are treated the same as absent fields, and that key casing differences
// (e.g. "type" vs "Type") do not cause false mismatches.
func canonicalizeJSON(v any) any {
	switch val := v.(type) {
	case map[string]any:
		cleaned := make(map[string]any, len(val))
		for k, child := range val {
			if child == nil {
				continue
			}
			cleaned[strings.ToLower(k)] = canonicalizeJSON(child)
		}
		return cleaned
	case []any:
		for i, child := range val {
			val[i] = canonicalizeJSON(child)
		}
		return val
	default:
		return v
	}
}

// toolDefinitionByName returns a map of ToolDefinition keyed by Name for
// efficient lookup during comparison.
func toolDefinitionByName(
	defs []*svcapitypes.ToolDefinition,
) map[string]*svcapitypes.ToolDefinition {
	m := make(map[string]*svcapitypes.ToolDefinition, len(defs))
	for _, td := range defs {
		if td.Name != nil {
			m[*td.Name] = td
		}
	}
	return m
}

// compareInlinePayloadToolDefinitions performs a semantic comparison of the
// InlinePayload ToolDefinition slices from two resources. It normalizes
// ordering by ToolDefinition Name and compares InputSchema/OutputSchema as
// parsed JSON so that key ordering and whitespace differences are ignored.
func compareInlinePayloadToolDefinitions(
	delta *ackcompare.Delta,
	a *resource,
	b *resource,
) {
	const fieldPath = "Spec.TargetConfiguration.Mcp.Lambda.ToolSchema.InlinePayload"

	aToolDefs := getMcpLambdaInlinePayload(a.ko.Spec.TargetConfiguration)
	bToolDefs := getMcpLambdaInlinePayload(b.ko.Spec.TargetConfiguration)

	// Both nil/empty — no difference.
	if len(aToolDefs) == 0 && len(bToolDefs) == 0 {
		return
	}

	// Length mismatch is always a difference.
	if len(aToolDefs) != len(bToolDefs) {
		delta.Add(fieldPath, aToolDefs, bToolDefs)
		return
	}

	// Build a lookup map for b keyed by Name so we can match elements
	// regardless of ordering.
	bByName := toolDefinitionByName(bToolDefs)

	for _, aTD := range aToolDefs {
		if aTD.Name == nil {
			continue
		}
		bTD, ok := bByName[*aTD.Name]
		if !ok {
			// A tool in a has no corresponding entry in b.
			delta.Add(fieldPath, aToolDefs, bToolDefs)
			return
		}

		// Compare Description.
		if ackcompare.HasNilDifference(aTD.Description, bTD.Description) {
			delta.Add(fieldPath, aToolDefs, bToolDefs)
			return
		}
		if aTD.Description != nil && *aTD.Description != *bTD.Description {
			delta.Add(fieldPath, aToolDefs, bToolDefs)
			return
		}

		// Compare InputSchema using normalized JSON.
		if ackcompare.HasNilDifference(aTD.InputSchema, bTD.InputSchema) {
			delta.Add(fieldPath, aToolDefs, bToolDefs)
			return
		}
		if aTD.InputSchema != nil && normalizeJSON(aTD.InputSchema) != normalizeJSON(bTD.InputSchema) {
			delta.Add(fieldPath, aToolDefs, bToolDefs)
			return
		}

		// Compare OutputSchema using normalized JSON.
		if ackcompare.HasNilDifference(aTD.OutputSchema, bTD.OutputSchema) {
			delta.Add(fieldPath, aToolDefs, bToolDefs)
			return
		}
		if aTD.OutputSchema != nil && normalizeJSON(aTD.OutputSchema) != normalizeJSON(bTD.OutputSchema) {
			delta.Add(fieldPath, aToolDefs, bToolDefs)
			return
		}
	}
}
