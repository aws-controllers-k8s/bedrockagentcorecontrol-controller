package memory

import (
	"context"
	"sort"

	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	"k8s.io/apimachinery/pkg/api/equality"

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

func compareMemoryStrategies(
	delta *ackcompare.Delta,
	a *resource,
	b *resource,
) {
	const fieldPath = "Spec.MemoryStrategies"

	aStrats := a.ko.Spec.MemoryStrategies
	bStrats := b.ko.Spec.MemoryStrategies

	if len(aStrats) == 0 && len(bStrats) == 0 {
		return
	}
	if len(aStrats) != len(bStrats) {
		delta.Add(fieldPath, aStrats, bStrats)
		return
	}

	aSorted := make([]*svcapitypes.MemoryStrategyInput, len(aStrats))
	copy(aSorted, aStrats)
	bSorted := make([]*svcapitypes.MemoryStrategyInput, len(bStrats))
	copy(bSorted, bStrats)

	sort.Slice(aSorted, func(i, j int) bool {
		return desiredStrategyName(aSorted[i]) < desiredStrategyName(aSorted[j])
	})
	sort.Slice(bSorted, func(i, j int) bool {
		return desiredStrategyName(bSorted[i]) < desiredStrategyName(bSorted[j])
	})

	for i := range aSorted {
		bNorm := stripAWSDefaults(aSorted[i], bSorted[i])
		if !equality.Semantic.Equalities.DeepEqual(aSorted[i], bNorm) {
			delta.Add(fieldPath, aStrats, bStrats)
			return
		}
	}
}

func isDefaultNamespaces(ns []*string) bool {
	return len(ns) == 1 && ns[0] != nil && *ns[0] == "default"
}

// stripAWSDefaults returns a copy of latest with AWS-populated default
// values removed, but only when the corresponding field in desired is
// nil/empty. This preserves the distinction when the user explicitly
// sets a value.
//
// AWS applies two default behaviors:
// 1. If neither namespaces nor namespaceTemplates is set, both default to ["default"].
// 2. If only one is set, the other is mirrored to the same value.
func stripAWSDefaults(desired, latest *svcapitypes.MemoryStrategyInput) *svcapitypes.MemoryStrategyInput {
	if latest == nil {
		return nil
	}
	out := latest.DeepCopy()
	if desired == nil {
		return out
	}

	if desired.EpisodicMemoryStrategy != nil && out.EpisodicMemoryStrategy != nil {
		normalizeNamespacePair(
			desired.EpisodicMemoryStrategy.Namespaces, desired.EpisodicMemoryStrategy.NamespaceTemplates,
			&out.EpisodicMemoryStrategy.Namespaces, &out.EpisodicMemoryStrategy.NamespaceTemplates,
		)
		if out.EpisodicMemoryStrategy.ReflectionConfiguration != nil {
			if desired.EpisodicMemoryStrategy.ReflectionConfiguration == nil {
				// User didn't set reflection config; strip if AWS only populated defaults
				rc := out.EpisodicMemoryStrategy.ReflectionConfiguration
				if (len(rc.Namespaces) == 0 || isDefaultNamespaces(rc.Namespaces)) &&
					(len(rc.NamespaceTemplates) == 0 || isDefaultNamespaces(rc.NamespaceTemplates)) {
					out.EpisodicMemoryStrategy.ReflectionConfiguration = nil
				}
			} else {
				normalizeNamespacePair(
					desired.EpisodicMemoryStrategy.ReflectionConfiguration.Namespaces,
					desired.EpisodicMemoryStrategy.ReflectionConfiguration.NamespaceTemplates,
					&out.EpisodicMemoryStrategy.ReflectionConfiguration.Namespaces,
					&out.EpisodicMemoryStrategy.ReflectionConfiguration.NamespaceTemplates,
				)
			}
		}
	}
	if desired.SemanticMemoryStrategy != nil && out.SemanticMemoryStrategy != nil {
		normalizeNamespacePair(
			desired.SemanticMemoryStrategy.Namespaces, desired.SemanticMemoryStrategy.NamespaceTemplates,
			&out.SemanticMemoryStrategy.Namespaces, &out.SemanticMemoryStrategy.NamespaceTemplates,
		)
	}
	if desired.SummaryMemoryStrategy != nil && out.SummaryMemoryStrategy != nil {
		normalizeNamespacePair(
			desired.SummaryMemoryStrategy.Namespaces, desired.SummaryMemoryStrategy.NamespaceTemplates,
			&out.SummaryMemoryStrategy.Namespaces, &out.SummaryMemoryStrategy.NamespaceTemplates,
		)
	}
	if desired.UserPreferenceMemoryStrategy != nil && out.UserPreferenceMemoryStrategy != nil {
		normalizeNamespacePair(
			desired.UserPreferenceMemoryStrategy.Namespaces, desired.UserPreferenceMemoryStrategy.NamespaceTemplates,
			&out.UserPreferenceMemoryStrategy.Namespaces, &out.UserPreferenceMemoryStrategy.NamespaceTemplates,
		)
	}
	if desired.CustomMemoryStrategy != nil && out.CustomMemoryStrategy != nil {
		normalizeNamespacePair(
			desired.CustomMemoryStrategy.Namespaces, desired.CustomMemoryStrategy.NamespaceTemplates,
			&out.CustomMemoryStrategy.Namespaces, &out.CustomMemoryStrategy.NamespaceTemplates,
		)
	}
	return out
}

// normalizeNamespacePair strips AWS-populated defaults from the latest
// namespaces/namespaceTemplates pair based on what the user set in desired.
//
// Rules:
// - If desired didn't set a field (nil/empty) and latest has ["default"], strip it.
// - If desired didn't set a field but latest mirrors the value from the other
//   field (because AWS copies one to the other), strip the mirrored value.
func normalizeNamespacePair(
	desiredNS, desiredNST []*string,
	latestNS, latestNST *[]*string,
) {
	if len(desiredNS) == 0 {
		if isDefaultNamespaces(*latestNS) {
			*latestNS = nil
		} else if len(desiredNST) > 0 && ptrSliceEqual(*latestNS, desiredNST) {
			*latestNS = nil
		}
	}
	if len(desiredNST) == 0 {
		if isDefaultNamespaces(*latestNST) {
			*latestNST = nil
		} else if len(desiredNS) > 0 && ptrSliceEqual(*latestNST, desiredNS) {
			*latestNST = nil
		}
	}
}

func ptrSliceEqual(a, b []*string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if (a[i] == nil) != (b[i] == nil) {
			return false
		}
		if a[i] != nil && *a[i] != *b[i] {
			return false
		}
	}
	return true
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
