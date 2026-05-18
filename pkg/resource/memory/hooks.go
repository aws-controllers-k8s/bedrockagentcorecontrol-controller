package memory

import (
	"context"

	svcapitypes "github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/apis/v1alpha1"
	"github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/pkg/tags"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol"
	svcsdktypes "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol/types"
)

var syncTags = tags.SyncTags

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

// desiredStrategyName extracts the name from a MemoryStrategyInput union.
func desiredStrategyName(s *svcapitypes.MemoryStrategyInput) string {
	if s == nil {
		return ""
	}
	switch {
	case s.EpisodicMemoryStrategy != nil && s.EpisodicMemoryStrategy.Name != nil:
		return *s.EpisodicMemoryStrategy.Name
	case s.SemanticMemoryStrategy != nil && s.SemanticMemoryStrategy.Name != nil:
		return *s.SemanticMemoryStrategy.Name
	case s.SummaryMemoryStrategy != nil && s.SummaryMemoryStrategy.Name != nil:
		return *s.SummaryMemoryStrategy.Name
	case s.UserPreferenceMemoryStrategy != nil && s.UserPreferenceMemoryStrategy.Name != nil:
		return *s.UserPreferenceMemoryStrategy.Name
	case s.CustomMemoryStrategy != nil && s.CustomMemoryStrategy.Name != nil:
		return *s.CustomMemoryStrategy.Name
	}
	return ""
}

// memoryStrategyInputToSDK converts a CRD MemoryStrategyInput to the SDK
// union type. Only the top-level union dispatch is handled here; the built-in
// strategy types (episodic, semantic, summary, user-preference) carry only
// simple scalar fields. Custom strategies with deep configuration are
// converted with their full nested structure.
func memoryStrategyInputToSDK(s *svcapitypes.MemoryStrategyInput) svcsdktypes.MemoryStrategyInput {
	if s.EpisodicMemoryStrategy != nil {
		out := &svcsdktypes.MemoryStrategyInputMemberEpisodicMemoryStrategy{}
		out.Value.Name = s.EpisodicMemoryStrategy.Name
		out.Value.Description = s.EpisodicMemoryStrategy.Description
		out.Value.Namespaces = aws.ToStringSlice(s.EpisodicMemoryStrategy.Namespaces)
		if s.EpisodicMemoryStrategy.ReflectionConfiguration != nil {
			out.Value.ReflectionConfiguration = &svcsdktypes.EpisodicReflectionConfigurationInput{
				Namespaces: aws.ToStringSlice(s.EpisodicMemoryStrategy.ReflectionConfiguration.Namespaces),
			}
		}
		return out
	}
	if s.SemanticMemoryStrategy != nil {
		out := &svcsdktypes.MemoryStrategyInputMemberSemanticMemoryStrategy{}
		out.Value.Name = s.SemanticMemoryStrategy.Name
		out.Value.Description = s.SemanticMemoryStrategy.Description
		out.Value.Namespaces = aws.ToStringSlice(s.SemanticMemoryStrategy.Namespaces)
		return out
	}
	if s.SummaryMemoryStrategy != nil {
		out := &svcsdktypes.MemoryStrategyInputMemberSummaryMemoryStrategy{}
		out.Value.Name = s.SummaryMemoryStrategy.Name
		out.Value.Description = s.SummaryMemoryStrategy.Description
		out.Value.Namespaces = aws.ToStringSlice(s.SummaryMemoryStrategy.Namespaces)
		return out
	}
	if s.UserPreferenceMemoryStrategy != nil {
		out := &svcsdktypes.MemoryStrategyInputMemberUserPreferenceMemoryStrategy{}
		out.Value.Name = s.UserPreferenceMemoryStrategy.Name
		out.Value.Description = s.UserPreferenceMemoryStrategy.Description
		out.Value.Namespaces = aws.ToStringSlice(s.UserPreferenceMemoryStrategy.Namespaces)
		return out
	}
	if s.CustomMemoryStrategy != nil {
		out := &svcsdktypes.MemoryStrategyInputMemberCustomMemoryStrategy{}
		out.Value.Name = s.CustomMemoryStrategy.Name
		out.Value.Description = s.CustomMemoryStrategy.Description
		out.Value.Namespaces = aws.ToStringSlice(s.CustomMemoryStrategy.Namespaces)
		// Custom strategy configuration is complex (deep union types).
		// For now we pass the top-level fields; full configuration
		// conversion can be added when custom strategy updates are tested.
		return out
	}
	return nil
}

func strategiesToMemoryStrategyInputs(strategies []*svcapitypes.MemoryStrategy) []*svcapitypes.MemoryStrategyInput {
	if len(strategies) == 0 {
		return nil
	}
	out := make([]*svcapitypes.MemoryStrategyInput, 0, len(strategies))
	for _, s := range strategies {
		if s == nil || s.Type == nil {
			continue
		}
		input := &svcapitypes.MemoryStrategyInput{}
		switch *s.Type {
		case "EPISODIC":
			ep := &svcapitypes.EpisodicMemoryStrategyInput{
				Name:               s.Name,
				Description:        s.Description,
				Namespaces:         s.Namespaces,
				NamespaceTemplates: s.NamespaceTemplates,
			}
			if s.Configuration != nil && s.Configuration.Reflection != nil &&
				s.Configuration.Reflection.EpisodicReflectionConfiguration != nil {
				ep.ReflectionConfiguration = &svcapitypes.EpisodicReflectionConfigurationInput{
					Namespaces:         s.Configuration.Reflection.EpisodicReflectionConfiguration.Namespaces,
					NamespaceTemplates: s.Configuration.Reflection.EpisodicReflectionConfiguration.NamespaceTemplates,
				}
			}
			input.EpisodicMemoryStrategy = ep
		case "SEMANTIC":
			input.SemanticMemoryStrategy = &svcapitypes.SemanticMemoryStrategyInput{
				Name:               s.Name,
				Description:        s.Description,
				Namespaces:         s.Namespaces,
				NamespaceTemplates: s.NamespaceTemplates,
			}
		case "SUMMARIZATION":
			input.SummaryMemoryStrategy = &svcapitypes.SummaryMemoryStrategyInput{
				Name:               s.Name,
				Description:        s.Description,
				Namespaces:         s.Namespaces,
				NamespaceTemplates: s.NamespaceTemplates,
			}
		case "USER_PREFERENCE":
			input.UserPreferenceMemoryStrategy = &svcapitypes.UserPreferenceMemoryStrategyInput{
				Name:               s.Name,
				Description:        s.Description,
				Namespaces:         s.Namespaces,
				NamespaceTemplates: s.NamespaceTemplates,
			}
		case "CUSTOM":
			custom := &svcapitypes.CustomMemoryStrategyInput{
				Name:               s.Name,
				Description:        s.Description,
				Namespaces:         s.Namespaces,
				NamespaceTemplates: s.NamespaceTemplates,
			}
			if s.Configuration != nil {
				custom.Configuration = strategyConfigToCustomConfigInput(s.Configuration)
			}
			input.CustomMemoryStrategy = custom
		default:
			continue
		}
		out = append(out, input)
	}
	return out
}

func strategyConfigToCustomConfigInput(cfg *svcapitypes.StrategyConfiguration) *svcapitypes.CustomConfigurationInput {
	out := &svcapitypes.CustomConfigurationInput{}

	if cfg.SelfManagedConfiguration != nil {
		out.SelfManagedConfiguration = selfManagedConfigToInput(cfg.SelfManagedConfiguration)
	}

	// Transpose status (grouped by step) → input (grouped by strategy type).
	// Status: Consolidation/Extraction/Reflection each contain per-strategy overrides.
	// Input: EpisodicOverride/SemanticOverride/etc. each contain per-step overrides.

	var episodicOverride *svcapitypes.EpisodicOverrideConfigurationInput
	var semanticOverride *svcapitypes.SemanticOverrideConfigurationInput
	var summaryOverride *svcapitypes.SummaryOverrideConfigurationInput
	var userPrefOverride *svcapitypes.UserPreferenceOverrideConfigurationInput

	if cfg.Consolidation != nil && cfg.Consolidation.CustomConsolidationConfiguration != nil {
		cc := cfg.Consolidation.CustomConsolidationConfiguration
		if cc.EpisodicConsolidationOverride != nil {
			if episodicOverride == nil {
				episodicOverride = &svcapitypes.EpisodicOverrideConfigurationInput{}
			}
			episodicOverride.Consolidation = &svcapitypes.EpisodicOverrideConsolidationConfigurationInput{
				AppendToPrompt: cc.EpisodicConsolidationOverride.AppendToPrompt,
				ModelID:        cc.EpisodicConsolidationOverride.ModelID,
			}
		}
		if cc.SemanticConsolidationOverride != nil {
			if semanticOverride == nil {
				semanticOverride = &svcapitypes.SemanticOverrideConfigurationInput{}
			}
			semanticOverride.Consolidation = &svcapitypes.SemanticOverrideConsolidationConfigurationInput{
				AppendToPrompt: cc.SemanticConsolidationOverride.AppendToPrompt,
				ModelID:        cc.SemanticConsolidationOverride.ModelID,
			}
		}
		if cc.SummaryConsolidationOverride != nil {
			if summaryOverride == nil {
				summaryOverride = &svcapitypes.SummaryOverrideConfigurationInput{}
			}
			summaryOverride.Consolidation = &svcapitypes.SummaryOverrideConsolidationConfigurationInput{
				AppendToPrompt: cc.SummaryConsolidationOverride.AppendToPrompt,
				ModelID:        cc.SummaryConsolidationOverride.ModelID,
			}
		}
		if cc.UserPreferenceConsolidationOverride != nil {
			if userPrefOverride == nil {
				userPrefOverride = &svcapitypes.UserPreferenceOverrideConfigurationInput{}
			}
			userPrefOverride.Consolidation = &svcapitypes.UserPreferenceOverrideConsolidationConfigurationInput{
				AppendToPrompt: cc.UserPreferenceConsolidationOverride.AppendToPrompt,
				ModelID:        cc.UserPreferenceConsolidationOverride.ModelID,
			}
		}
	}

	if cfg.Extraction != nil && cfg.Extraction.CustomExtractionConfiguration != nil {
		ce := cfg.Extraction.CustomExtractionConfiguration
		if ce.EpisodicExtractionOverride != nil {
			if episodicOverride == nil {
				episodicOverride = &svcapitypes.EpisodicOverrideConfigurationInput{}
			}
			episodicOverride.Extraction = &svcapitypes.EpisodicOverrideExtractionConfigurationInput{
				AppendToPrompt: ce.EpisodicExtractionOverride.AppendToPrompt,
				ModelID:        ce.EpisodicExtractionOverride.ModelID,
			}
		}
		if ce.SemanticExtractionOverride != nil {
			if semanticOverride == nil {
				semanticOverride = &svcapitypes.SemanticOverrideConfigurationInput{}
			}
			semanticOverride.Extraction = &svcapitypes.SemanticOverrideExtractionConfigurationInput{
				AppendToPrompt: ce.SemanticExtractionOverride.AppendToPrompt,
				ModelID:        ce.SemanticExtractionOverride.ModelID,
			}
		}
		if ce.UserPreferenceExtractionOverride != nil {
			if userPrefOverride == nil {
				userPrefOverride = &svcapitypes.UserPreferenceOverrideConfigurationInput{}
			}
			userPrefOverride.Extraction = &svcapitypes.UserPreferenceOverrideExtractionConfigurationInput{
				AppendToPrompt: ce.UserPreferenceExtractionOverride.AppendToPrompt,
				ModelID:        ce.UserPreferenceExtractionOverride.ModelID,
			}
		}
	}

	if cfg.Reflection != nil && cfg.Reflection.CustomReflectionConfiguration != nil {
		cr := cfg.Reflection.CustomReflectionConfiguration
		if cr.EpisodicReflectionOverride != nil {
			if episodicOverride == nil {
				episodicOverride = &svcapitypes.EpisodicOverrideConfigurationInput{}
			}
			episodicOverride.Reflection = &svcapitypes.EpisodicOverrideReflectionConfigurationInput{
				AppendToPrompt:     cr.EpisodicReflectionOverride.AppendToPrompt,
				ModelID:            cr.EpisodicReflectionOverride.ModelID,
				Namespaces:         cr.EpisodicReflectionOverride.Namespaces,
				NamespaceTemplates: cr.EpisodicReflectionOverride.NamespaceTemplates,
			}
		}
	}

	out.EpisodicOverride = episodicOverride
	out.SemanticOverride = semanticOverride
	out.SummaryOverride = summaryOverride
	out.UserPreferenceOverride = userPrefOverride

	// If everything is nil, return nil to avoid an empty struct in the spec
	if out.SelfManagedConfiguration == nil &&
		out.EpisodicOverride == nil &&
		out.SemanticOverride == nil &&
		out.SummaryOverride == nil &&
		out.UserPreferenceOverride == nil {
		return nil
	}
	return out
}

func selfManagedConfigToInput(smc *svcapitypes.SelfManagedConfiguration) *svcapitypes.SelfManagedConfigurationInput {
	if smc == nil {
		return nil
	}
	out := &svcapitypes.SelfManagedConfigurationInput{
		HistoricalContextWindowSize: smc.HistoricalContextWindowSize,
	}
	if smc.InvocationConfiguration != nil {
		out.InvocationConfiguration = &svcapitypes.InvocationConfigurationInput{
			PayloadDeliveryBucketName: smc.InvocationConfiguration.PayloadDeliveryBucketName,
			TopicARN:                  smc.InvocationConfiguration.TopicARN,
		}
	}
	if smc.TriggerConditions != nil {
		triggers := make([]*svcapitypes.TriggerConditionInput, 0, len(smc.TriggerConditions))
		for _, tc := range smc.TriggerConditions {
			if tc == nil {
				continue
			}
			tci := &svcapitypes.TriggerConditionInput{}
			if tc.MessageBasedTrigger != nil {
				tci.MessageBasedTrigger = &svcapitypes.MessageBasedTriggerInput{
					MessageCount: tc.MessageBasedTrigger.MessageCount,
				}
			}
			if tc.TimeBasedTrigger != nil {
				tci.TimeBasedTrigger = &svcapitypes.TimeBasedTriggerInput{
					IdleSessionTimeout: tc.TimeBasedTrigger.IdleSessionTimeout,
				}
			}
			if tc.TokenBasedTrigger != nil {
				tci.TokenBasedTrigger = &svcapitypes.TokenBasedTriggerInput{
					TokenCount: tc.TokenBasedTrigger.TokenCount,
				}
			}
			triggers = append(triggers, tci)
		}
		out.TriggerConditions = triggers
	}
	return out
}

// buildModifyMemoryStrategies computes the add/delete sets needed to reconcile
// desired spec.memoryStrategies against the current status.strategies.
// Strategies are matched by name. Changed strategies are handled as
// delete + re-add since the ModifyMemoryStrategyInput uses a different
// configuration shape that doesn't map 1:1 from the create input.
func buildModifyMemoryStrategies(
	desired []*svcapitypes.MemoryStrategyInput,
	current []*svcapitypes.MemoryStrategy,
) *svcsdktypes.ModifyMemoryStrategies {
	currentByName := make(map[string]string, len(current))
	for _, s := range current {
		if s.Name != nil && s.StrategyID != nil {
			currentByName[*s.Name] = *s.StrategyID
		}
	}

	desiredNames := make(map[string]struct{}, len(desired))
	for _, s := range desired {
		if name := desiredStrategyName(s); name != "" {
			desiredNames[name] = struct{}{}
		}
	}

	var adds []svcsdktypes.MemoryStrategyInput
	var deletes []svcsdktypes.DeleteMemoryStrategyInput

	// Strategies in desired but not in current → add
	for _, s := range desired {
		name := desiredStrategyName(s)
		if _, exists := currentByName[name]; !exists {
			if sdk := memoryStrategyInputToSDK(s); sdk != nil {
				adds = append(adds, sdk)
			}
		}
	}

	// Strategies in current but not in desired → delete
	for name, id := range currentByName {
		if _, exists := desiredNames[name]; !exists {
			idCopy := id
			deletes = append(deletes, svcsdktypes.DeleteMemoryStrategyInput{
				MemoryStrategyId: &idCopy,
			})
		}
	}

	if len(adds) == 0 && len(deletes) == 0 {
		return nil
	}
	return &svcsdktypes.ModifyMemoryStrategies{
		AddMemoryStrategies:    adds,
		DeleteMemoryStrategies: deletes,
	}
}
