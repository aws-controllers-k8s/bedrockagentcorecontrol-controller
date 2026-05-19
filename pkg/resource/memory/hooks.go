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
//   - If desired didn't set a field (nil/empty) and latest has ["default"], strip it.
//   - If desired didn't set a field but latest mirrors the value from the other
//     field (because AWS copies one to the other), strip the mirrored value.
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
		ep := s.EpisodicMemoryStrategy
		out := &svcsdktypes.MemoryStrategyInputMemberEpisodicMemoryStrategy{}
		out.Value.Name = ep.Name
		out.Value.Description = ep.Description
		if len(ep.Namespaces) > 0 {
			out.Value.Namespaces = aws.ToStringSlice(ep.Namespaces)
		}
		if len(ep.NamespaceTemplates) > 0 {
			out.Value.NamespaceTemplates = aws.ToStringSlice(ep.NamespaceTemplates)
		}
		if ep.ReflectionConfiguration != nil {
			rc := &svcsdktypes.EpisodicReflectionConfigurationInput{}
			if len(ep.ReflectionConfiguration.Namespaces) > 0 {
				rc.Namespaces = aws.ToStringSlice(ep.ReflectionConfiguration.Namespaces)
			}
			if len(ep.ReflectionConfiguration.NamespaceTemplates) > 0 {
				rc.NamespaceTemplates = aws.ToStringSlice(ep.ReflectionConfiguration.NamespaceTemplates)
			}
			out.Value.ReflectionConfiguration = rc
		}
		return out
	}
	if s.SemanticMemoryStrategy != nil {
		sem := s.SemanticMemoryStrategy
		out := &svcsdktypes.MemoryStrategyInputMemberSemanticMemoryStrategy{}
		out.Value.Name = sem.Name
		out.Value.Description = sem.Description
		if len(sem.Namespaces) > 0 {
			out.Value.Namespaces = aws.ToStringSlice(sem.Namespaces)
		}
		if len(sem.NamespaceTemplates) > 0 {
			out.Value.NamespaceTemplates = aws.ToStringSlice(sem.NamespaceTemplates)
		}
		return out
	}
	if s.SummaryMemoryStrategy != nil {
		sum := s.SummaryMemoryStrategy
		out := &svcsdktypes.MemoryStrategyInputMemberSummaryMemoryStrategy{}
		out.Value.Name = sum.Name
		out.Value.Description = sum.Description
		if len(sum.Namespaces) > 0 {
			out.Value.Namespaces = aws.ToStringSlice(sum.Namespaces)
		}
		if len(sum.NamespaceTemplates) > 0 {
			out.Value.NamespaceTemplates = aws.ToStringSlice(sum.NamespaceTemplates)
		}
		return out
	}
	if s.UserPreferenceMemoryStrategy != nil {
		up := s.UserPreferenceMemoryStrategy
		out := &svcsdktypes.MemoryStrategyInputMemberUserPreferenceMemoryStrategy{}
		out.Value.Name = up.Name
		out.Value.Description = up.Description
		if len(up.Namespaces) > 0 {
			out.Value.Namespaces = aws.ToStringSlice(up.Namespaces)
		}
		if len(up.NamespaceTemplates) > 0 {
			out.Value.NamespaceTemplates = aws.ToStringSlice(up.NamespaceTemplates)
		}
		return out
	}
	if s.CustomMemoryStrategy != nil {
		custom := s.CustomMemoryStrategy
		out := &svcsdktypes.MemoryStrategyInputMemberCustomMemoryStrategy{}
		out.Value.Name = custom.Name
		out.Value.Description = custom.Description
		if len(custom.Namespaces) > 0 {
			out.Value.Namespaces = aws.ToStringSlice(custom.Namespaces)
		}
		if len(custom.NamespaceTemplates) > 0 {
			out.Value.NamespaceTemplates = aws.ToStringSlice(custom.NamespaceTemplates)
		}
		if custom.Configuration != nil {
			out.Value.Configuration = customConfigInputToSDK(custom.Configuration)
		}
		return out
	}
	return nil
}

func customConfigInputToSDK(cfg *svcapitypes.CustomConfigurationInput) svcsdktypes.CustomConfigurationInput {
	if cfg.SelfManagedConfiguration != nil {
		smc := cfg.SelfManagedConfiguration
		sdkSmc := svcsdktypes.SelfManagedConfigurationInput{
			InvocationConfiguration: &svcsdktypes.InvocationConfigurationInput{
				TopicArn:                  smc.InvocationConfiguration.TopicARN,
				PayloadDeliveryBucketName: smc.InvocationConfiguration.PayloadDeliveryBucketName,
			},
		}
		if smc.HistoricalContextWindowSize != nil {
			v := int32(*smc.HistoricalContextWindowSize)
			sdkSmc.HistoricalContextWindowSize = &v
		}
		for _, tc := range smc.TriggerConditions {
			if tc == nil {
				continue
			}
			if tc.MessageBasedTrigger != nil {
				v := int32(*tc.MessageBasedTrigger.MessageCount)
				sdkSmc.TriggerConditions = append(sdkSmc.TriggerConditions,
					&svcsdktypes.TriggerConditionInputMemberMessageBasedTrigger{
						Value: svcsdktypes.MessageBasedTriggerInput{MessageCount: &v},
					})
			}
			if tc.TimeBasedTrigger != nil {
				v := int32(*tc.TimeBasedTrigger.IdleSessionTimeout)
				sdkSmc.TriggerConditions = append(sdkSmc.TriggerConditions,
					&svcsdktypes.TriggerConditionInputMemberTimeBasedTrigger{
						Value: svcsdktypes.TimeBasedTriggerInput{IdleSessionTimeout: &v},
					})
			}
			if tc.TokenBasedTrigger != nil {
				v := int32(*tc.TokenBasedTrigger.TokenCount)
				sdkSmc.TriggerConditions = append(sdkSmc.TriggerConditions,
					&svcsdktypes.TriggerConditionInputMemberTokenBasedTrigger{
						Value: svcsdktypes.TokenBasedTriggerInput{TokenCount: &v},
					})
			}
		}
		return &svcsdktypes.CustomConfigurationInputMemberSelfManagedConfiguration{
			Value: sdkSmc,
		}
	}
	if cfg.EpisodicOverride != nil {
		eo := cfg.EpisodicOverride
		sdkEo := svcsdktypes.EpisodicOverrideConfigurationInput{}
		if eo.Consolidation != nil {
			sdkEo.Consolidation = &svcsdktypes.EpisodicOverrideConsolidationConfigurationInput{
				AppendToPrompt: eo.Consolidation.AppendToPrompt,
				ModelId:        eo.Consolidation.ModelID,
			}
		}
		if eo.Extraction != nil {
			sdkEo.Extraction = &svcsdktypes.EpisodicOverrideExtractionConfigurationInput{
				AppendToPrompt: eo.Extraction.AppendToPrompt,
				ModelId:        eo.Extraction.ModelID,
			}
		}
		if eo.Reflection != nil {
			sdkEo.Reflection = &svcsdktypes.EpisodicOverrideReflectionConfigurationInput{
				AppendToPrompt:     eo.Reflection.AppendToPrompt,
				ModelId:            eo.Reflection.ModelID,
				Namespaces:         aws.ToStringSlice(eo.Reflection.Namespaces),
				NamespaceTemplates: aws.ToStringSlice(eo.Reflection.NamespaceTemplates),
			}
		}
		return &svcsdktypes.CustomConfigurationInputMemberEpisodicOverride{
			Value: sdkEo,
		}
	}
	if cfg.SemanticOverride != nil {
		so := cfg.SemanticOverride
		sdkSo := svcsdktypes.SemanticOverrideConfigurationInput{}
		if so.Consolidation != nil {
			sdkSo.Consolidation = &svcsdktypes.SemanticOverrideConsolidationConfigurationInput{
				AppendToPrompt: so.Consolidation.AppendToPrompt,
				ModelId:        so.Consolidation.ModelID,
			}
		}
		if so.Extraction != nil {
			sdkSo.Extraction = &svcsdktypes.SemanticOverrideExtractionConfigurationInput{
				AppendToPrompt: so.Extraction.AppendToPrompt,
				ModelId:        so.Extraction.ModelID,
			}
		}
		return &svcsdktypes.CustomConfigurationInputMemberSemanticOverride{
			Value: sdkSo,
		}
	}
	if cfg.SummaryOverride != nil {
		so := cfg.SummaryOverride
		sdkSo := svcsdktypes.SummaryOverrideConfigurationInput{}
		if so.Consolidation != nil {
			sdkSo.Consolidation = &svcsdktypes.SummaryOverrideConsolidationConfigurationInput{
				AppendToPrompt: so.Consolidation.AppendToPrompt,
				ModelId:        so.Consolidation.ModelID,
			}
		}
		return &svcsdktypes.CustomConfigurationInputMemberSummaryOverride{
			Value: sdkSo,
		}
	}
	if cfg.UserPreferenceOverride != nil {
		upo := cfg.UserPreferenceOverride
		sdkUpo := svcsdktypes.UserPreferenceOverrideConfigurationInput{}
		if upo.Consolidation != nil {
			sdkUpo.Consolidation = &svcsdktypes.UserPreferenceOverrideConsolidationConfigurationInput{
				AppendToPrompt: upo.Consolidation.AppendToPrompt,
				ModelId:        upo.Consolidation.ModelID,
			}
		}
		if upo.Extraction != nil {
			sdkUpo.Extraction = &svcsdktypes.UserPreferenceOverrideExtractionConfigurationInput{
				AppendToPrompt: upo.Extraction.AppendToPrompt,
				ModelId:        upo.Extraction.ModelID,
			}
		}
		return &svcsdktypes.CustomConfigurationInputMemberUserPreferenceOverride{
			Value: sdkUpo,
		}
	}
	return nil
}

func sdkStrategiesToMemoryStrategyInputs(strategies []svcsdktypes.MemoryStrategy) []*svcapitypes.MemoryStrategyInput {
	if len(strategies) == 0 {
		return nil
	}
	out := make([]*svcapitypes.MemoryStrategyInput, 0, len(strategies))
	for i := range strategies {
		s := &strategies[i]
		input := &svcapitypes.MemoryStrategyInput{}
		switch s.Type {
		case svcsdktypes.MemoryStrategyTypeEpisodic:
			ep := &svcapitypes.EpisodicMemoryStrategyInput{
				Name:        s.Name,
				Description: s.Description,
			}
			if s.Namespaces != nil {
				ep.Namespaces = aws.StringSlice(s.Namespaces)
			}
			if s.NamespaceTemplates != nil {
				ep.NamespaceTemplates = aws.StringSlice(s.NamespaceTemplates)
			}
			if s.Configuration != nil && s.Configuration.Reflection != nil {
				if rc, ok := s.Configuration.Reflection.(*svcsdktypes.ReflectionConfigurationMemberEpisodicReflectionConfiguration); ok {
					refCfg := &svcapitypes.EpisodicReflectionConfigurationInput{}
					if rc.Value.Namespaces != nil {
						refCfg.Namespaces = aws.StringSlice(rc.Value.Namespaces)
					}
					if rc.Value.NamespaceTemplates != nil {
						refCfg.NamespaceTemplates = aws.StringSlice(rc.Value.NamespaceTemplates)
					}
					ep.ReflectionConfiguration = refCfg
				}
			}
			input.EpisodicMemoryStrategy = ep
		case svcsdktypes.MemoryStrategyTypeSemantic:
			sem := &svcapitypes.SemanticMemoryStrategyInput{
				Name:        s.Name,
				Description: s.Description,
			}
			if s.Namespaces != nil {
				sem.Namespaces = aws.StringSlice(s.Namespaces)
			}
			if s.NamespaceTemplates != nil {
				sem.NamespaceTemplates = aws.StringSlice(s.NamespaceTemplates)
			}
			input.SemanticMemoryStrategy = sem
		case svcsdktypes.MemoryStrategyTypeSummarization:
			sum := &svcapitypes.SummaryMemoryStrategyInput{
				Name:        s.Name,
				Description: s.Description,
			}
			if s.Namespaces != nil {
				sum.Namespaces = aws.StringSlice(s.Namespaces)
			}
			if s.NamespaceTemplates != nil {
				sum.NamespaceTemplates = aws.StringSlice(s.NamespaceTemplates)
			}
			input.SummaryMemoryStrategy = sum
		case svcsdktypes.MemoryStrategyTypeUserPreference:
			up := &svcapitypes.UserPreferenceMemoryStrategyInput{
				Name:        s.Name,
				Description: s.Description,
			}
			if s.Namespaces != nil {
				up.Namespaces = aws.StringSlice(s.Namespaces)
			}
			if s.NamespaceTemplates != nil {
				up.NamespaceTemplates = aws.StringSlice(s.NamespaceTemplates)
			}
			input.UserPreferenceMemoryStrategy = up
		case svcsdktypes.MemoryStrategyTypeCustom:
			custom := &svcapitypes.CustomMemoryStrategyInput{
				Name:        s.Name,
				Description: s.Description,
			}
			if s.Namespaces != nil {
				custom.Namespaces = aws.StringSlice(s.Namespaces)
			}
			if s.NamespaceTemplates != nil {
				custom.NamespaceTemplates = aws.StringSlice(s.NamespaceTemplates)
			}
			if s.Configuration != nil {
				custom.Configuration = sdkStrategyConfigToCustomConfigInput(s.Configuration)
			}
			input.CustomMemoryStrategy = custom
		default:
			continue
		}
		out = append(out, input)
	}
	return out
}

func sdkStrategyConfigToCustomConfigInput(cfg *svcsdktypes.StrategyConfiguration) *svcapitypes.CustomConfigurationInput {
	out := &svcapitypes.CustomConfigurationInput{}

	if cfg.SelfManagedConfiguration != nil {
		smc := cfg.SelfManagedConfiguration
		smcInput := &svcapitypes.SelfManagedConfigurationInput{}
		if smc.HistoricalContextWindowSize != nil {
			v := int64(*smc.HistoricalContextWindowSize)
			smcInput.HistoricalContextWindowSize = &v
		}
		if smc.InvocationConfiguration != nil {
			smcInput.InvocationConfiguration = &svcapitypes.InvocationConfigurationInput{
				PayloadDeliveryBucketName: smc.InvocationConfiguration.PayloadDeliveryBucketName,
				TopicARN:                  smc.InvocationConfiguration.TopicArn,
			}
		}
		if smc.TriggerConditions != nil {
			triggers := make([]*svcapitypes.TriggerConditionInput, 0, len(smc.TriggerConditions))
			for _, tc := range smc.TriggerConditions {
				tci := &svcapitypes.TriggerConditionInput{}
				switch v := tc.(type) {
				case *svcsdktypes.TriggerConditionMemberMessageBasedTrigger:
					msgCount := int64(*v.Value.MessageCount)
					tci.MessageBasedTrigger = &svcapitypes.MessageBasedTriggerInput{MessageCount: &msgCount}
				case *svcsdktypes.TriggerConditionMemberTimeBasedTrigger:
					timeout := int64(*v.Value.IdleSessionTimeout)
					tci.TimeBasedTrigger = &svcapitypes.TimeBasedTriggerInput{IdleSessionTimeout: &timeout}
				case *svcsdktypes.TriggerConditionMemberTokenBasedTrigger:
					count := int64(*v.Value.TokenCount)
					tci.TokenBasedTrigger = &svcapitypes.TokenBasedTriggerInput{TokenCount: &count}
				}
				triggers = append(triggers, tci)
			}
			smcInput.TriggerConditions = triggers
		}
		out.SelfManagedConfiguration = smcInput
	}

	var episodicOverride *svcapitypes.EpisodicOverrideConfigurationInput
	var semanticOverride *svcapitypes.SemanticOverrideConfigurationInput
	var summaryOverride *svcapitypes.SummaryOverrideConfigurationInput
	var userPrefOverride *svcapitypes.UserPreferenceOverrideConfigurationInput

	if cfg.Consolidation != nil {
		if cc, ok := cfg.Consolidation.(*svcsdktypes.ConsolidationConfigurationMemberCustomConsolidationConfiguration); ok {
			switch v := cc.Value.(type) {
			case *svcsdktypes.CustomConsolidationConfigurationMemberEpisodicConsolidationOverride:
				if episodicOverride == nil {
					episodicOverride = &svcapitypes.EpisodicOverrideConfigurationInput{}
				}
				episodicOverride.Consolidation = &svcapitypes.EpisodicOverrideConsolidationConfigurationInput{
					AppendToPrompt: v.Value.AppendToPrompt,
					ModelID:        v.Value.ModelId,
				}
			case *svcsdktypes.CustomConsolidationConfigurationMemberSemanticConsolidationOverride:
				if semanticOverride == nil {
					semanticOverride = &svcapitypes.SemanticOverrideConfigurationInput{}
				}
				semanticOverride.Consolidation = &svcapitypes.SemanticOverrideConsolidationConfigurationInput{
					AppendToPrompt: v.Value.AppendToPrompt,
					ModelID:        v.Value.ModelId,
				}
			case *svcsdktypes.CustomConsolidationConfigurationMemberSummaryConsolidationOverride:
				if summaryOverride == nil {
					summaryOverride = &svcapitypes.SummaryOverrideConfigurationInput{}
				}
				summaryOverride.Consolidation = &svcapitypes.SummaryOverrideConsolidationConfigurationInput{
					AppendToPrompt: v.Value.AppendToPrompt,
					ModelID:        v.Value.ModelId,
				}
			case *svcsdktypes.CustomConsolidationConfigurationMemberUserPreferenceConsolidationOverride:
				if userPrefOverride == nil {
					userPrefOverride = &svcapitypes.UserPreferenceOverrideConfigurationInput{}
				}
				userPrefOverride.Consolidation = &svcapitypes.UserPreferenceOverrideConsolidationConfigurationInput{
					AppendToPrompt: v.Value.AppendToPrompt,
					ModelID:        v.Value.ModelId,
				}
			}
		}
	}

	if cfg.Extraction != nil {
		if ce, ok := cfg.Extraction.(*svcsdktypes.ExtractionConfigurationMemberCustomExtractionConfiguration); ok {
			switch v := ce.Value.(type) {
			case *svcsdktypes.CustomExtractionConfigurationMemberEpisodicExtractionOverride:
				if episodicOverride == nil {
					episodicOverride = &svcapitypes.EpisodicOverrideConfigurationInput{}
				}
				episodicOverride.Extraction = &svcapitypes.EpisodicOverrideExtractionConfigurationInput{
					AppendToPrompt: v.Value.AppendToPrompt,
					ModelID:        v.Value.ModelId,
				}
			case *svcsdktypes.CustomExtractionConfigurationMemberSemanticExtractionOverride:
				if semanticOverride == nil {
					semanticOverride = &svcapitypes.SemanticOverrideConfigurationInput{}
				}
				semanticOverride.Extraction = &svcapitypes.SemanticOverrideExtractionConfigurationInput{
					AppendToPrompt: v.Value.AppendToPrompt,
					ModelID:        v.Value.ModelId,
				}
			case *svcsdktypes.CustomExtractionConfigurationMemberUserPreferenceExtractionOverride:
				if userPrefOverride == nil {
					userPrefOverride = &svcapitypes.UserPreferenceOverrideConfigurationInput{}
				}
				userPrefOverride.Extraction = &svcapitypes.UserPreferenceOverrideExtractionConfigurationInput{
					AppendToPrompt: v.Value.AppendToPrompt,
					ModelID:        v.Value.ModelId,
				}
			}
		}
	}

	if cfg.Reflection != nil {
		if cr, ok := cfg.Reflection.(*svcsdktypes.ReflectionConfigurationMemberCustomReflectionConfiguration); ok {
			if ero, ok := cr.Value.(*svcsdktypes.CustomReflectionConfigurationMemberEpisodicReflectionOverride); ok {
				if episodicOverride == nil {
					episodicOverride = &svcapitypes.EpisodicOverrideConfigurationInput{}
				}
				episodicOverride.Reflection = &svcapitypes.EpisodicOverrideReflectionConfigurationInput{
					AppendToPrompt: ero.Value.AppendToPrompt,
					ModelID:        ero.Value.ModelId,
				}
				if ero.Value.Namespaces != nil {
					episodicOverride.Reflection.Namespaces = aws.StringSlice(ero.Value.Namespaces)
				}
				if ero.Value.NamespaceTemplates != nil {
					episodicOverride.Reflection.NamespaceTemplates = aws.StringSlice(ero.Value.NamespaceTemplates)
				}
			}
		}
	}

	out.EpisodicOverride = episodicOverride
	out.SemanticOverride = semanticOverride
	out.SummaryOverride = summaryOverride
	out.UserPreferenceOverride = userPrefOverride

	if out.SelfManagedConfiguration == nil &&
		out.EpisodicOverride == nil &&
		out.SemanticOverride == nil &&
		out.SummaryOverride == nil &&
		out.UserPreferenceOverride == nil {
		return nil
	}
	return out
}

// buildModifyMemoryStrategies computes the add/modify/delete sets needed to
// reconcile desired spec.memoryStrategies against latest spec.memoryStrategies.
// Strategies are matched by name. Strategy IDs are looked up from status.
func buildModifyMemoryStrategies(
	desired []*svcapitypes.MemoryStrategyInput,
	latest []*svcapitypes.MemoryStrategyInput,
	statusStrategies []*svcapitypes.MemoryStrategy,
) *svcsdktypes.ModifyMemoryStrategies {
	// Build strategy ID lookup from status
	strategyIDByName := make(map[string]string, len(statusStrategies))
	for _, s := range statusStrategies {
		if s.Name != nil && s.StrategyID != nil {
			strategyIDByName[*s.Name] = *s.StrategyID
		}
	}

	latestByName := make(map[string]*svcapitypes.MemoryStrategyInput, len(latest))
	for _, s := range latest {
		if name := desiredStrategyName(s); name != "" {
			latestByName[name] = s
		}
	}

	desiredNames := make(map[string]struct{}, len(desired))
	for _, s := range desired {
		if name := desiredStrategyName(s); name != "" {
			desiredNames[name] = struct{}{}
		}
	}

	var adds []svcsdktypes.MemoryStrategyInput
	var modifies []svcsdktypes.ModifyMemoryStrategyInput
	var deletes []svcsdktypes.DeleteMemoryStrategyInput

	for _, s := range desired {
		name := desiredStrategyName(s)
		latestStrategy, exists := latestByName[name]
		if !exists {
			if sdk := memoryStrategyInputToSDK(s); sdk != nil {
				adds = append(adds, sdk)
			}
		} else {
			strategyID := strategyIDByName[name]
			if mod := buildModifyStrategyInput(s, latestStrategy, strategyID); mod != nil {
				modifies = append(modifies, *mod)
			}
		}
	}

	for name := range latestByName {
		if _, exists := desiredNames[name]; !exists {
			if id, ok := strategyIDByName[name]; ok {
				idCopy := id
				deletes = append(deletes, svcsdktypes.DeleteMemoryStrategyInput{
					MemoryStrategyId: &idCopy,
				})
			}
		}
	}

	if len(adds) == 0 && len(deletes) == 0 && len(modifies) == 0 {
		return nil
	}
	return &svcsdktypes.ModifyMemoryStrategies{
		AddMemoryStrategies:    adds,
		DeleteMemoryStrategies: deletes,
		ModifyMemoryStrategies: modifies,
	}
}

// buildModifyStrategyInput builds a ModifyMemoryStrategyInput if the desired
// strategy differs from the latest (already normalized from spec). Returns nil
// if no change is needed.
func buildModifyStrategyInput(
	desired *svcapitypes.MemoryStrategyInput,
	latest *svcapitypes.MemoryStrategyInput,
	strategyID string,
) *svcsdktypes.ModifyMemoryStrategyInput {
	normalized := stripAWSDefaults(desired, latest)
	if equality.Semantic.Equalities.DeepEqual(desired, normalized) {
		return nil
	}

	mod := &svcsdktypes.ModifyMemoryStrategyInput{
		MemoryStrategyId: &strategyID,
	}

	var desiredDesc *string
	var desiredNS []*string
	var desiredNST []*string

	switch {
	case desired.EpisodicMemoryStrategy != nil:
		desiredDesc = desired.EpisodicMemoryStrategy.Description
		desiredNS = desired.EpisodicMemoryStrategy.Namespaces
		desiredNST = desired.EpisodicMemoryStrategy.NamespaceTemplates
	case desired.SemanticMemoryStrategy != nil:
		desiredDesc = desired.SemanticMemoryStrategy.Description
		desiredNS = desired.SemanticMemoryStrategy.Namespaces
		desiredNST = desired.SemanticMemoryStrategy.NamespaceTemplates
	case desired.SummaryMemoryStrategy != nil:
		desiredDesc = desired.SummaryMemoryStrategy.Description
		desiredNS = desired.SummaryMemoryStrategy.Namespaces
		desiredNST = desired.SummaryMemoryStrategy.NamespaceTemplates
	case desired.UserPreferenceMemoryStrategy != nil:
		desiredDesc = desired.UserPreferenceMemoryStrategy.Description
		desiredNS = desired.UserPreferenceMemoryStrategy.Namespaces
		desiredNST = desired.UserPreferenceMemoryStrategy.NamespaceTemplates
	case desired.CustomMemoryStrategy != nil:
		desiredDesc = desired.CustomMemoryStrategy.Description
		desiredNS = desired.CustomMemoryStrategy.Namespaces
		desiredNST = desired.CustomMemoryStrategy.NamespaceTemplates
	}

	mod.Description = desiredDesc
	if len(desiredNS) > 0 {
		mod.Namespaces = aws.ToStringSlice(desiredNS)
	}
	if len(desiredNST) > 0 {
		mod.NamespaceTemplates = aws.ToStringSlice(desiredNST)
	}

	if modCfg := buildModifyStrategyConfiguration(desired, latest); modCfg != nil {
		mod.Configuration = modCfg
	}

	return mod
}

func ptrStringEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func buildModifyStrategyConfiguration(
	desired *svcapitypes.MemoryStrategyInput,
	latest *svcapitypes.MemoryStrategyInput,
) *svcsdktypes.ModifyStrategyConfiguration {
	cfg := &svcsdktypes.ModifyStrategyConfiguration{}
	hasChange := false

	// Episodic reflection configuration
	if desired.EpisodicMemoryStrategy != nil && desired.EpisodicMemoryStrategy.ReflectionConfiguration != nil {
		desiredRef := desired.EpisodicMemoryStrategy.ReflectionConfiguration
		var latestRefNS, latestRefNST []*string
		if latest != nil && latest.EpisodicMemoryStrategy != nil &&
			latest.EpisodicMemoryStrategy.ReflectionConfiguration != nil {
			latestRefNS = latest.EpisodicMemoryStrategy.ReflectionConfiguration.Namespaces
			latestRefNST = latest.EpisodicMemoryStrategy.ReflectionConfiguration.NamespaceTemplates
		}
		normalizeNamespacePair(desiredRef.Namespaces, desiredRef.NamespaceTemplates, &latestRefNS, &latestRefNST)

		refChanged := false
		refInput := svcsdktypes.EpisodicReflectionConfigurationInput{}
		if len(desiredRef.Namespaces) > 0 && !ptrSliceEqual(desiredRef.Namespaces, latestRefNS) {
			refInput.Namespaces = aws.ToStringSlice(desiredRef.Namespaces)
			refChanged = true
		}
		if len(desiredRef.NamespaceTemplates) > 0 && !ptrSliceEqual(desiredRef.NamespaceTemplates, latestRefNST) {
			refInput.NamespaceTemplates = aws.ToStringSlice(desiredRef.NamespaceTemplates)
			refChanged = true
		}
		if refChanged {
			cfg.Reflection = &svcsdktypes.ModifyReflectionConfigurationMemberEpisodicReflectionConfiguration{
				Value: refInput,
			}
			hasChange = true
		}
	}

	// Custom strategy configuration
	if desired.CustomMemoryStrategy != nil && desired.CustomMemoryStrategy.Configuration != nil {
		customCfg := desired.CustomMemoryStrategy.Configuration

		if customCfg.SelfManagedConfiguration != nil {
			smc := customCfg.SelfManagedConfiguration
			modSmc := &svcsdktypes.ModifySelfManagedConfiguration{}
			if smc.HistoricalContextWindowSize != nil {
				v := int32(*smc.HistoricalContextWindowSize)
				modSmc.HistoricalContextWindowSize = &v
			}
			if smc.InvocationConfiguration != nil {
				modSmc.InvocationConfiguration = &svcsdktypes.ModifyInvocationConfigurationInput{
					PayloadDeliveryBucketName: smc.InvocationConfiguration.PayloadDeliveryBucketName,
					TopicArn:                  smc.InvocationConfiguration.TopicARN,
				}
			}
			if smc.TriggerConditions != nil {
				for _, tc := range smc.TriggerConditions {
					if tc == nil {
						continue
					}
					if tc.MessageBasedTrigger != nil {
						v := int32(*tc.MessageBasedTrigger.MessageCount)
						modSmc.TriggerConditions = append(modSmc.TriggerConditions,
							&svcsdktypes.TriggerConditionInputMemberMessageBasedTrigger{
								Value: svcsdktypes.MessageBasedTriggerInput{MessageCount: &v},
							})
					}
					if tc.TimeBasedTrigger != nil {
						v := int32(*tc.TimeBasedTrigger.IdleSessionTimeout)
						modSmc.TriggerConditions = append(modSmc.TriggerConditions,
							&svcsdktypes.TriggerConditionInputMemberTimeBasedTrigger{
								Value: svcsdktypes.TimeBasedTriggerInput{IdleSessionTimeout: &v},
							})
					}
					if tc.TokenBasedTrigger != nil {
						v := int32(*tc.TokenBasedTrigger.TokenCount)
						modSmc.TriggerConditions = append(modSmc.TriggerConditions,
							&svcsdktypes.TriggerConditionInputMemberTokenBasedTrigger{
								Value: svcsdktypes.TokenBasedTriggerInput{TokenCount: &v},
							})
					}
				}
			}
			cfg.SelfManagedConfiguration = modSmc
			hasChange = true
		}

		if customCfg.EpisodicOverride != nil {
			cfg.Consolidation = buildModifyConsolidation(customCfg)
			cfg.Extraction = buildModifyExtraction(customCfg)
			cfg.Reflection = buildModifyCustomReflection(customCfg)
			hasChange = true
		} else if customCfg.SemanticOverride != nil || customCfg.SummaryOverride != nil || customCfg.UserPreferenceOverride != nil {
			cfg.Consolidation = buildModifyConsolidation(customCfg)
			cfg.Extraction = buildModifyExtraction(customCfg)
			hasChange = true
		}
	}

	if !hasChange {
		return nil
	}
	return cfg
}

func buildModifyConsolidation(cfg *svcapitypes.CustomConfigurationInput) svcsdktypes.ModifyConsolidationConfiguration {
	// Each override type is a separate union member — only one can be active
	switch {
	case cfg.EpisodicOverride != nil && cfg.EpisodicOverride.Consolidation != nil:
		return &svcsdktypes.ModifyConsolidationConfigurationMemberCustomConsolidationConfiguration{
			Value: &svcsdktypes.CustomConsolidationConfigurationInputMemberEpisodicConsolidationOverride{
				Value: svcsdktypes.EpisodicOverrideConsolidationConfigurationInput{
					AppendToPrompt: cfg.EpisodicOverride.Consolidation.AppendToPrompt,
					ModelId:        cfg.EpisodicOverride.Consolidation.ModelID,
				},
			},
		}
	case cfg.SemanticOverride != nil && cfg.SemanticOverride.Consolidation != nil:
		return &svcsdktypes.ModifyConsolidationConfigurationMemberCustomConsolidationConfiguration{
			Value: &svcsdktypes.CustomConsolidationConfigurationInputMemberSemanticConsolidationOverride{
				Value: svcsdktypes.SemanticOverrideConsolidationConfigurationInput{
					AppendToPrompt: cfg.SemanticOverride.Consolidation.AppendToPrompt,
					ModelId:        cfg.SemanticOverride.Consolidation.ModelID,
				},
			},
		}
	case cfg.SummaryOverride != nil && cfg.SummaryOverride.Consolidation != nil:
		return &svcsdktypes.ModifyConsolidationConfigurationMemberCustomConsolidationConfiguration{
			Value: &svcsdktypes.CustomConsolidationConfigurationInputMemberSummaryConsolidationOverride{
				Value: svcsdktypes.SummaryOverrideConsolidationConfigurationInput{
					AppendToPrompt: cfg.SummaryOverride.Consolidation.AppendToPrompt,
					ModelId:        cfg.SummaryOverride.Consolidation.ModelID,
				},
			},
		}
	case cfg.UserPreferenceOverride != nil && cfg.UserPreferenceOverride.Consolidation != nil:
		return &svcsdktypes.ModifyConsolidationConfigurationMemberCustomConsolidationConfiguration{
			Value: &svcsdktypes.CustomConsolidationConfigurationInputMemberUserPreferenceConsolidationOverride{
				Value: svcsdktypes.UserPreferenceOverrideConsolidationConfigurationInput{
					AppendToPrompt: cfg.UserPreferenceOverride.Consolidation.AppendToPrompt,
					ModelId:        cfg.UserPreferenceOverride.Consolidation.ModelID,
				},
			},
		}
	}
	return nil
}

func buildModifyExtraction(cfg *svcapitypes.CustomConfigurationInput) svcsdktypes.ModifyExtractionConfiguration {
	switch {
	case cfg.EpisodicOverride != nil && cfg.EpisodicOverride.Extraction != nil:
		return &svcsdktypes.ModifyExtractionConfigurationMemberCustomExtractionConfiguration{
			Value: &svcsdktypes.CustomExtractionConfigurationInputMemberEpisodicExtractionOverride{
				Value: svcsdktypes.EpisodicOverrideExtractionConfigurationInput{
					AppendToPrompt: cfg.EpisodicOverride.Extraction.AppendToPrompt,
					ModelId:        cfg.EpisodicOverride.Extraction.ModelID,
				},
			},
		}
	case cfg.SemanticOverride != nil && cfg.SemanticOverride.Extraction != nil:
		return &svcsdktypes.ModifyExtractionConfigurationMemberCustomExtractionConfiguration{
			Value: &svcsdktypes.CustomExtractionConfigurationInputMemberSemanticExtractionOverride{
				Value: svcsdktypes.SemanticOverrideExtractionConfigurationInput{
					AppendToPrompt: cfg.SemanticOverride.Extraction.AppendToPrompt,
					ModelId:        cfg.SemanticOverride.Extraction.ModelID,
				},
			},
		}
	case cfg.UserPreferenceOverride != nil && cfg.UserPreferenceOverride.Extraction != nil:
		return &svcsdktypes.ModifyExtractionConfigurationMemberCustomExtractionConfiguration{
			Value: &svcsdktypes.CustomExtractionConfigurationInputMemberUserPreferenceExtractionOverride{
				Value: svcsdktypes.UserPreferenceOverrideExtractionConfigurationInput{
					AppendToPrompt: cfg.UserPreferenceOverride.Extraction.AppendToPrompt,
					ModelId:        cfg.UserPreferenceOverride.Extraction.ModelID,
				},
			},
		}
	}
	return nil
}

func buildModifyCustomReflection(cfg *svcapitypes.CustomConfigurationInput) svcsdktypes.ModifyReflectionConfiguration {
	if cfg.EpisodicOverride == nil || cfg.EpisodicOverride.Reflection == nil {
		return nil
	}
	r := cfg.EpisodicOverride.Reflection
	return &svcsdktypes.ModifyReflectionConfigurationMemberCustomReflectionConfiguration{
		Value: &svcsdktypes.CustomReflectionConfigurationInputMemberEpisodicReflectionOverride{
			Value: svcsdktypes.EpisodicOverrideReflectionConfigurationInput{
				AppendToPrompt:     r.AppendToPrompt,
				ModelId:            r.ModelID,
				Namespaces:         aws.ToStringSlice(r.Namespaces),
				NamespaceTemplates: aws.ToStringSlice(r.NamespaceTemplates),
			},
		},
	}
}
