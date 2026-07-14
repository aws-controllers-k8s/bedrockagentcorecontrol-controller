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

	svcapitypes "github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/apis/v1alpha1"
	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"
	svcsdkdocument "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol/document"
	svcsdktypes "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol/types"
	"k8s.io/apimachinery/pkg/api/equality"
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

// schemasEqual compares two JSON schema strings by unmarshaling them into
// SDK SchemaDefinition structs and using equality.Semantic.Equalities.DeepEqual.
// This naturally handles differences in key casing (Go's json.Unmarshal is
// case-insensitive) and explicit null fields (they unmarshal to nil pointers,
// same as absent fields).
func schemasEqual(a, b *string) bool {
	sdA, errA := stringToSchemaDefinition(a)
	sdB, errB := stringToSchemaDefinition(b)
	if errA != nil || errB != nil {
		// If either fails to parse, fall back to raw string comparison.
		return ptrStringVal(a) == ptrStringVal(b)
	}
	if sdA == nil && sdB == nil {
		return true
	}
	if sdA == nil || sdB == nil {
		return false
	}
	return equality.Semantic.Equalities.DeepEqual(sdA, sdB)
}

// ptrStringVal safely dereferences a *string, returning "" if nil.
func ptrStringVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// -----------------------------------------------------------------------------
// Connector target ParameterValues (smithy document <-> JSON string)
//
// ConnectorConfiguration.ParameterValues is a smithy document (arbitrary JSON)
// which has no CRD representation, so it is exposed as a JSON string and
// marshaled/unmarshaled here. The API requires the field on every connector
// configuration -- omitting it is rejected with "Connector configurations must
// not be empty" -- so it must round-trip rather than be dropped.
// -----------------------------------------------------------------------------

// stringToDocument unmarshals a JSON string into a smithy document suitable for
// the SDK ConnectorConfiguration.ParameterValues field.
func stringToDocument(s *string) (svcsdkdocument.Interface, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	var v interface{}
	if err := json.Unmarshal([]byte(*s), &v); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ParameterValues JSON: %w", err)
	}
	return svcsdkdocument.NewLazyDocument(v), nil
}

// documentToString marshals a smithy document (as returned by the API) into a
// JSON string. Inverse of stringToDocument, used when reading the resource back.
func documentToString(d svcsdkdocument.Interface) (*string, error) {
	if d == nil {
		return nil, nil
	}
	// MarshalSmithyDocument yields the document's JSON byte representation and
	// works for both lazy (outbound) and response (inbound) documents.
	b, err := d.MarshalSmithyDocument()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ParameterValues document: %w", err)
	}
	s := string(b)
	return &s, nil
}

// getMcpConnectorConfigurations returns the connector configurations from the
// TargetConfiguration if the target is an MCP connector target. Returns nil if
// the path doesn't apply.
func getMcpConnectorConfigurations(tc *svcapitypes.TargetConfiguration) []*svcapitypes.ConnectorConfiguration {
	if tc == nil || tc.Mcp == nil || tc.Mcp.Connector == nil {
		return nil
	}
	return tc.Mcp.Connector.Configurations
}

// getSDKConnectorConfigurations extracts the connector configuration slice from
// an SDK TargetConfiguration union, returning nil if the path doesn't match.
func getSDKConnectorConfigurations(tc svcsdktypes.TargetConfiguration) []svcsdktypes.ConnectorConfiguration {
	mcpMember, ok := tc.(*svcsdktypes.TargetConfigurationMemberMcp)
	if !ok || mcpMember == nil {
		return nil
	}
	connMember, ok := mcpMember.Value.(*svcsdktypes.McpTargetConfigurationMemberConnector)
	if !ok || connMember == nil {
		return nil
	}
	return connMember.Value.Configurations
}

// setConnectorParameterValuesOnInput sets each SDK configuration's
// ParameterValues document by unmarshaling the JSON string from the CR.
func setConnectorParameterValuesOnInput(
	crCfgs []*svcapitypes.ConnectorConfiguration,
	sdkCfgs []svcsdktypes.ConnectorConfiguration,
) error {
	for i, cfg := range crCfgs {
		if i >= len(sdkCfgs) {
			break
		}
		doc, err := stringToDocument(cfg.ParameterValues)
		if err != nil {
			return err
		}
		sdkCfgs[i].ParameterValues = doc
	}
	return nil
}

// setConnectorParameterValuesOnCreateInput populates connector ParameterValues
// documents on the CreateGatewayTarget input from the CR's JSON strings.
func setConnectorParameterValuesOnCreateInput(desired *resource, input *svcsdk.CreateGatewayTargetInput) error {
	crCfgs := getMcpConnectorConfigurations(desired.ko.Spec.TargetConfiguration)
	if crCfgs == nil {
		return nil
	}
	sdkCfgs := getSDKConnectorConfigurations(input.TargetConfiguration)
	if sdkCfgs == nil {
		return nil
	}
	return setConnectorParameterValuesOnInput(crCfgs, sdkCfgs)
}

// setConnectorParameterValuesOnUpdateInput populates connector ParameterValues
// documents on the UpdateGatewayTarget input from the CR's JSON strings.
func setConnectorParameterValuesOnUpdateInput(desired *resource, input *svcsdk.UpdateGatewayTargetInput) error {
	crCfgs := getMcpConnectorConfigurations(desired.ko.Spec.TargetConfiguration)
	if crCfgs == nil {
		return nil
	}
	sdkCfgs := getSDKConnectorConfigurations(input.TargetConfiguration)
	if sdkCfgs == nil {
		return nil
	}
	return setConnectorParameterValuesOnInput(crCfgs, sdkCfgs)
}

// setConnectorParameterValuesFromSDKResponse reads the SDK GetGatewayTarget
// response and populates the connector ParameterValues JSON strings on the CR.
func setConnectorParameterValuesFromSDKResponse(
	ko *svcapitypes.GatewayTarget,
	resp *svcsdk.GetGatewayTargetOutput,
) error {
	if resp.TargetConfiguration == nil {
		return nil
	}
	sdkCfgs := getSDKConnectorConfigurations(resp.TargetConfiguration)
	if sdkCfgs == nil {
		return nil
	}
	crCfgs := getMcpConnectorConfigurations(ko.Spec.TargetConfiguration)
	if crCfgs == nil {
		return nil
	}
	for i, sdkCfg := range sdkCfgs {
		if i >= len(crCfgs) {
			break
		}
		s, err := documentToString(sdkCfg.ParameterValues)
		if err != nil {
			return err
		}
		crCfgs[i].ParameterValues = s
	}
	return nil
}

// jsonStringsEqual compares two JSON strings semantically (ignoring key ordering
// and whitespace). A nil/empty string and a JSON object that unmarshals to an
// empty value compare equal, so an omitted-vs-"{}" round trip is not a diff.
func jsonStringsEqual(a, b *string) bool {
	var va, vb interface{}
	errA := json.Unmarshal([]byte(ptrStringVal(a)), &va)
	errB := json.Unmarshal([]byte(ptrStringVal(b)), &vb)
	if errA != nil || errB != nil {
		return ptrStringVal(a) == ptrStringVal(b)
	}
	return equality.Semantic.Equalities.DeepEqual(va, vb)
}

// connectorConfigByName returns a map of ConnectorConfiguration keyed by Name
// for order-independent lookup during comparison.
func connectorConfigByName(
	cfgs []*svcapitypes.ConnectorConfiguration,
) map[string]*svcapitypes.ConnectorConfiguration {
	m := make(map[string]*svcapitypes.ConnectorConfiguration, len(cfgs))
	for _, cfg := range cfgs {
		if cfg.Name != nil {
			m[*cfg.Name] = cfg
		}
	}
	return m
}

// compareConnectorParameterValues performs a semantic comparison of the
// connector Configurations slices, matching by Name and comparing Description
// and the ParameterValues JSON semantically (ignoring key ordering/whitespace
// and treating an omitted-vs-"{}" round trip as equal). It owns the whole
// Configurations comparison because the generated delta is disabled for it via
// `compare: is_ignored: true`; without this a document round trip would drive a
// perpetual update loop.
func compareConnectorParameterValues(
	delta *ackcompare.Delta,
	a *resource,
	b *resource,
) {
	const fieldPath = "Spec.TargetConfiguration.Mcp.Connector.Configurations"

	aCfgs := getMcpConnectorConfigurations(a.ko.Spec.TargetConfiguration)
	bCfgs := getMcpConnectorConfigurations(b.ko.Spec.TargetConfiguration)
	if len(aCfgs) == 0 && len(bCfgs) == 0 {
		return
	}
	if len(aCfgs) != len(bCfgs) {
		delta.Add(fieldPath, aCfgs, bCfgs)
		return
	}

	bByName := connectorConfigByName(bCfgs)
	for _, aCfg := range aCfgs {
		if aCfg.Name == nil {
			continue
		}
		bCfg, ok := bByName[*aCfg.Name]
		if !ok {
			delta.Add(fieldPath, aCfgs, bCfgs)
			return
		}
		if ackcompare.HasNilDifference(aCfg.Description, bCfg.Description) ||
			(aCfg.Description != nil && *aCfg.Description != *bCfg.Description) {
			delta.Add(fieldPath, aCfgs, bCfgs)
			return
		}
		if !jsonStringsEqual(aCfg.ParameterValues, bCfg.ParameterValues) {
			delta.Add(fieldPath, aCfgs, bCfgs)
			return
		}
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

		// Compare InputSchema using typed SchemaDefinition comparison.
		if ackcompare.HasNilDifference(aTD.InputSchema, bTD.InputSchema) {
			delta.Add(fieldPath, aToolDefs, bToolDefs)
			return
		}
		if aTD.InputSchema != nil && !schemasEqual(aTD.InputSchema, bTD.InputSchema) {
			delta.Add(fieldPath, aToolDefs, bToolDefs)
			return
		}

		// Compare OutputSchema using typed SchemaDefinition comparison.
		if ackcompare.HasNilDifference(aTD.OutputSchema, bTD.OutputSchema) {
			delta.Add(fieldPath, aToolDefs, bToolDefs)
			return
		}
		if aTD.OutputSchema != nil && !schemasEqual(aTD.OutputSchema, bTD.OutputSchema) {
			delta.Add(fieldPath, aToolDefs, bToolDefs)
			return
		}
	}
}
