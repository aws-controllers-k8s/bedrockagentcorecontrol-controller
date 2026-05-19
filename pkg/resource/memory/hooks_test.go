package memory

import (
	"testing"

	svcapitypes "github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/apis/v1alpha1"
	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdktypes "github.com/aws/aws-sdk-go-v2/service/bedrockagentcorecontrol/types"
)

func TestBuildModifyMemoryStrategies_NilWhenNoChanges(t *testing.T) {
	desired := []*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:               aws.String("ep1"),
			NamespaceTemplates: []*string{aws.String("default")},
		}},
	}
	latest := []*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:               aws.String("ep1"),
			Namespaces:         []*string{aws.String("default")},
			NamespaceTemplates: []*string{aws.String("default")},
		}},
	}
	status := []*svcapitypes.MemoryStrategy{
		{Name: aws.String("ep1"), StrategyID: aws.String("id-1")},
	}

	result := buildModifyMemoryStrategies(desired, latest, status)

	if result != nil {
		t.Errorf("expected nil when no changes, got %+v", result)
	}
}

func TestBuildModifyMemoryStrategies_AddNewStrategy(t *testing.T) {
	desired := []*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name: aws.String("ep1"),
		}},
		{SemanticMemoryStrategy: &svcapitypes.SemanticMemoryStrategyInput{
			Name:               aws.String("sem1"),
			NamespaceTemplates: []*string{aws.String("my-ns")},
		}},
	}
	latest := []*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name: aws.String("ep1"),
		}},
	}
	status := []*svcapitypes.MemoryStrategy{
		{Name: aws.String("ep1"), StrategyID: aws.String("id-1")},
	}

	result := buildModifyMemoryStrategies(desired, latest, status)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.AddMemoryStrategies) != 1 {
		t.Errorf("expected 1 add, got %d", len(result.AddMemoryStrategies))
	}
	if len(result.DeleteMemoryStrategies) != 0 {
		t.Errorf("expected 0 deletes, got %d", len(result.DeleteMemoryStrategies))
	}
	if len(result.ModifyMemoryStrategies) != 0 {
		t.Errorf("expected 0 modifies, got %d", len(result.ModifyMemoryStrategies))
	}
}

func TestBuildModifyMemoryStrategies_DeleteStrategy(t *testing.T) {
	desired := []*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name: aws.String("ep1"),
		}},
	}
	latest := []*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name: aws.String("ep1"),
		}},
		{SemanticMemoryStrategy: &svcapitypes.SemanticMemoryStrategyInput{
			Name: aws.String("sem1"),
		}},
	}
	status := []*svcapitypes.MemoryStrategy{
		{Name: aws.String("ep1"), StrategyID: aws.String("id-1")},
		{Name: aws.String("sem1"), StrategyID: aws.String("id-2")},
	}

	result := buildModifyMemoryStrategies(desired, latest, status)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.AddMemoryStrategies) != 0 {
		t.Errorf("expected 0 adds, got %d", len(result.AddMemoryStrategies))
	}
	if len(result.DeleteMemoryStrategies) != 1 {
		t.Errorf("expected 1 delete, got %d", len(result.DeleteMemoryStrategies))
	}
	if result.DeleteMemoryStrategies[0].MemoryStrategyId == nil ||
		*result.DeleteMemoryStrategies[0].MemoryStrategyId != "id-2" {
		t.Errorf("expected delete of id-2, got %v", result.DeleteMemoryStrategies[0].MemoryStrategyId)
	}
	if len(result.ModifyMemoryStrategies) != 0 {
		t.Errorf("expected 0 modifies, got %d", len(result.ModifyMemoryStrategies))
	}
}

func TestBuildModifyMemoryStrategies_ModifyDescription(t *testing.T) {
	desired := []*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:        aws.String("ep1"),
			Description: aws.String("updated desc"),
		}},
	}
	latest := []*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:        aws.String("ep1"),
			Description: aws.String("old desc"),
		}},
	}
	status := []*svcapitypes.MemoryStrategy{
		{Name: aws.String("ep1"), StrategyID: aws.String("id-1")},
	}

	result := buildModifyMemoryStrategies(desired, latest, status)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.ModifyMemoryStrategies) != 1 {
		t.Fatalf("expected 1 modify, got %d", len(result.ModifyMemoryStrategies))
	}
	mod := result.ModifyMemoryStrategies[0]
	if *mod.MemoryStrategyId != "id-1" {
		t.Errorf("expected strategy id id-1, got %s", *mod.MemoryStrategyId)
	}
	if mod.Description == nil || *mod.Description != "updated desc" {
		t.Errorf("expected description 'updated desc', got %v", mod.Description)
	}
}

func TestBuildModifyMemoryStrategies_ModifyNamespaceTemplates(t *testing.T) {
	desired := []*svcapitypes.MemoryStrategyInput{
		{SemanticMemoryStrategy: &svcapitypes.SemanticMemoryStrategyInput{
			Name:               aws.String("sem1"),
			NamespaceTemplates: []*string{aws.String("new-ns")},
		}},
	}
	latest := []*svcapitypes.MemoryStrategyInput{
		{SemanticMemoryStrategy: &svcapitypes.SemanticMemoryStrategyInput{
			Name:               aws.String("sem1"),
			NamespaceTemplates: []*string{aws.String("old-ns")},
		}},
	}
	status := []*svcapitypes.MemoryStrategy{
		{Name: aws.String("sem1"), StrategyID: aws.String("id-1")},
	}

	result := buildModifyMemoryStrategies(desired, latest, status)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.ModifyMemoryStrategies) != 1 {
		t.Fatalf("expected 1 modify, got %d", len(result.ModifyMemoryStrategies))
	}
	mod := result.ModifyMemoryStrategies[0]
	if len(mod.NamespaceTemplates) != 1 || mod.NamespaceTemplates[0] != "new-ns" {
		t.Errorf("expected namespaceTemplates [new-ns], got %v", mod.NamespaceTemplates)
	}
}

func TestBuildModifyMemoryStrategies_NoModifyForDefaultNamespaces(t *testing.T) {
	// desired has nil namespaces, latest has AWS-populated defaults — no modify
	desired := []*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:               aws.String("ep1"),
			Description:        aws.String("desc"),
			NamespaceTemplates: []*string{aws.String("my-ns")},
		}},
	}
	latest := []*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:               aws.String("ep1"),
			Description:        aws.String("desc"),
			Namespaces:         []*string{aws.String("my-ns")},
			NamespaceTemplates: []*string{aws.String("my-ns")},
		}},
	}
	status := []*svcapitypes.MemoryStrategy{
		{Name: aws.String("ep1"), StrategyID: aws.String("id-1")},
	}

	result := buildModifyMemoryStrategies(desired, latest, status)

	if result != nil {
		t.Errorf("expected nil when only difference is mirrored namespaces, got %+v", result)
	}
}

func TestBuildModifyMemoryStrategies_MixedOperations(t *testing.T) {
	desired := []*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:        aws.String("ep1"),
			Description: aws.String("changed"),
		}},
		{SummaryMemoryStrategy: &svcapitypes.SummaryMemoryStrategyInput{
			Name:               aws.String("new-sum"),
			NamespaceTemplates: []*string{aws.String("ns1")},
		}},
	}
	latest := []*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:        aws.String("ep1"),
			Description: aws.String("original"),
		}},
		{SemanticMemoryStrategy: &svcapitypes.SemanticMemoryStrategyInput{
			Name: aws.String("sem1"),
		}},
	}
	status := []*svcapitypes.MemoryStrategy{
		{Name: aws.String("ep1"), StrategyID: aws.String("id-1")},
		{Name: aws.String("sem1"), StrategyID: aws.String("id-2")},
	}

	result := buildModifyMemoryStrategies(desired, latest, status)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.AddMemoryStrategies) != 1 {
		t.Errorf("expected 1 add, got %d", len(result.AddMemoryStrategies))
	}
	if len(result.DeleteMemoryStrategies) != 1 {
		t.Errorf("expected 1 delete, got %d", len(result.DeleteMemoryStrategies))
	}
	if len(result.ModifyMemoryStrategies) != 1 {
		t.Errorf("expected 1 modify, got %d", len(result.ModifyMemoryStrategies))
	}
}

func TestBuildModifyMemoryStrategies_AddSetsNamespaceTemplates(t *testing.T) {
	desired := []*svcapitypes.MemoryStrategyInput{
		{SemanticMemoryStrategy: &svcapitypes.SemanticMemoryStrategyInput{
			Name:               aws.String("sem1"),
			NamespaceTemplates: []*string{aws.String("my-ns")},
		}},
	}
	latest := []*svcapitypes.MemoryStrategyInput{}
	status := []*svcapitypes.MemoryStrategy{}

	result := buildModifyMemoryStrategies(desired, latest, status)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.AddMemoryStrategies) != 1 {
		t.Fatalf("expected 1 add, got %d", len(result.AddMemoryStrategies))
	}
	add, ok := result.AddMemoryStrategies[0].(*svcsdktypes.MemoryStrategyInputMemberSemanticMemoryStrategy)
	if !ok {
		t.Fatal("expected SemanticMemoryStrategy member")
	}
	if len(add.Value.NamespaceTemplates) != 1 || add.Value.NamespaceTemplates[0] != "my-ns" {
		t.Errorf("expected namespaceTemplates [my-ns], got %v", add.Value.NamespaceTemplates)
	}
	if len(add.Value.Namespaces) != 0 {
		t.Errorf("expected empty namespaces when not set, got %v", add.Value.Namespaces)
	}
}

func TestBuildModifyMemoryStrategies_BothEmpty(t *testing.T) {
	result := buildModifyMemoryStrategies(nil, nil, nil)
	if result != nil {
		t.Errorf("expected nil for empty inputs, got %+v", result)
	}
}

func TestBuildModifyMemoryStrategies_DeleteAll(t *testing.T) {
	desired := []*svcapitypes.MemoryStrategyInput{}
	latest := []*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{Name: aws.String("ep1")}},
		{SemanticMemoryStrategy: &svcapitypes.SemanticMemoryStrategyInput{Name: aws.String("sem1")}},
	}
	status := []*svcapitypes.MemoryStrategy{
		{Name: aws.String("ep1"), StrategyID: aws.String("id-1")},
		{Name: aws.String("sem1"), StrategyID: aws.String("id-2")},
	}

	result := buildModifyMemoryStrategies(desired, latest, status)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.DeleteMemoryStrategies) != 2 {
		t.Errorf("expected 2 deletes, got %d", len(result.DeleteMemoryStrategies))
	}
	if len(result.AddMemoryStrategies) != 0 {
		t.Errorf("expected 0 adds, got %d", len(result.AddMemoryStrategies))
	}
	if len(result.ModifyMemoryStrategies) != 0 {
		t.Errorf("expected 0 modifies, got %d", len(result.ModifyMemoryStrategies))
	}
}

func TestBuildModifyMemoryStrategies_AddAll(t *testing.T) {
	desired := []*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{Name: aws.String("ep1")}},
		{SemanticMemoryStrategy: &svcapitypes.SemanticMemoryStrategyInput{Name: aws.String("sem1")}},
	}
	latest := []*svcapitypes.MemoryStrategyInput{}
	status := []*svcapitypes.MemoryStrategy{}

	result := buildModifyMemoryStrategies(desired, latest, status)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.AddMemoryStrategies) != 2 {
		t.Errorf("expected 2 adds, got %d", len(result.AddMemoryStrategies))
	}
	if len(result.DeleteMemoryStrategies) != 0 {
		t.Errorf("expected 0 deletes, got %d", len(result.DeleteMemoryStrategies))
	}
	if len(result.ModifyMemoryStrategies) != 0 {
		t.Errorf("expected 0 modifies, got %d", len(result.ModifyMemoryStrategies))
	}
}

func TestBuildModifyMemoryStrategies_RenameIsDeletePlusAdd(t *testing.T) {
	desired := []*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{Name: aws.String("ep-renamed")}},
	}
	latest := []*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{Name: aws.String("ep-original")}},
	}
	status := []*svcapitypes.MemoryStrategy{
		{Name: aws.String("ep-original"), StrategyID: aws.String("id-1")},
	}

	result := buildModifyMemoryStrategies(desired, latest, status)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.AddMemoryStrategies) != 1 {
		t.Errorf("expected 1 add for renamed strategy, got %d", len(result.AddMemoryStrategies))
	}
	if len(result.DeleteMemoryStrategies) != 1 {
		t.Errorf("expected 1 delete for old name, got %d", len(result.DeleteMemoryStrategies))
	}
	if len(result.ModifyMemoryStrategies) != 0 {
		t.Errorf("expected 0 modifies, got %d", len(result.ModifyMemoryStrategies))
	}
}

func TestBuildModifyMemoryStrategies_CustomWithConfigUnchanged(t *testing.T) {
	cfg := &svcapitypes.CustomConfigurationInput{
		SemanticOverride: &svcapitypes.SemanticOverrideConfigurationInput{
			Consolidation: &svcapitypes.SemanticOverrideConsolidationConfigurationInput{
				AppendToPrompt: aws.String("prompt text"),
				ModelID:        aws.String("us.amazon.nova-lite-v1:0"),
			},
			Extraction: &svcapitypes.SemanticOverrideExtractionConfigurationInput{
				AppendToPrompt: aws.String("extract text"),
				ModelID:        aws.String("us.amazon.nova-lite-v1:0"),
			},
		},
	}
	desired := []*svcapitypes.MemoryStrategyInput{
		{CustomMemoryStrategy: &svcapitypes.CustomMemoryStrategyInput{
			Name:          aws.String("custom1"),
			Description:   aws.String("desc"),
			Configuration: cfg,
		}},
	}
	latest := []*svcapitypes.MemoryStrategyInput{
		{CustomMemoryStrategy: &svcapitypes.CustomMemoryStrategyInput{
			Name:          aws.String("custom1"),
			Description:   aws.String("desc"),
			Configuration: cfg.DeepCopy(),
		}},
	}
	status := []*svcapitypes.MemoryStrategy{
		{Name: aws.String("custom1"), StrategyID: aws.String("id-1")},
	}

	result := buildModifyMemoryStrategies(desired, latest, status)

	if result != nil {
		t.Errorf("expected nil when custom config unchanged, got adds=%d deletes=%d modifies=%d",
			len(result.AddMemoryStrategies), len(result.DeleteMemoryStrategies), len(result.ModifyMemoryStrategies))
	}
}

func TestBuildModifyMemoryStrategies_CustomConfigChanged(t *testing.T) {
	desired := []*svcapitypes.MemoryStrategyInput{
		{CustomMemoryStrategy: &svcapitypes.CustomMemoryStrategyInput{
			Name:        aws.String("custom1"),
			Description: aws.String("desc"),
			Configuration: &svcapitypes.CustomConfigurationInput{
				SemanticOverride: &svcapitypes.SemanticOverrideConfigurationInput{
					Consolidation: &svcapitypes.SemanticOverrideConsolidationConfigurationInput{
						AppendToPrompt: aws.String("NEW prompt"),
						ModelID:        aws.String("us.amazon.nova-lite-v1:0"),
					},
				},
			},
		}},
	}
	latest := []*svcapitypes.MemoryStrategyInput{
		{CustomMemoryStrategy: &svcapitypes.CustomMemoryStrategyInput{
			Name:        aws.String("custom1"),
			Description: aws.String("desc"),
			Configuration: &svcapitypes.CustomConfigurationInput{
				SemanticOverride: &svcapitypes.SemanticOverrideConfigurationInput{
					Consolidation: &svcapitypes.SemanticOverrideConsolidationConfigurationInput{
						AppendToPrompt: aws.String("OLD prompt"),
						ModelID:        aws.String("us.amazon.nova-lite-v1:0"),
					},
				},
			},
		}},
	}
	status := []*svcapitypes.MemoryStrategy{
		{Name: aws.String("custom1"), StrategyID: aws.String("id-1")},
	}

	result := buildModifyMemoryStrategies(desired, latest, status)

	if result == nil {
		t.Fatal("expected non-nil result for changed custom config")
	}
	if len(result.ModifyMemoryStrategies) != 1 {
		t.Fatalf("expected 1 modify, got %d", len(result.ModifyMemoryStrategies))
	}
	mod := result.ModifyMemoryStrategies[0]
	if *mod.MemoryStrategyId != "id-1" {
		t.Errorf("expected strategy id id-1, got %s", *mod.MemoryStrategyId)
	}
	if mod.Configuration == nil {
		t.Error("expected configuration to be set on modify")
	}
}

func TestBuildModifyMemoryStrategies_EpisodicReflectionChanged(t *testing.T) {
	desired := []*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:               aws.String("ep1"),
			NamespaceTemplates: []*string{aws.String("ns1")},
			ReflectionConfiguration: &svcapitypes.EpisodicReflectionConfigurationInput{
				NamespaceTemplates: []*string{aws.String("new-ref-ns")},
			},
		}},
	}
	latest := []*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:               aws.String("ep1"),
			NamespaceTemplates: []*string{aws.String("ns1")},
			ReflectionConfiguration: &svcapitypes.EpisodicReflectionConfigurationInput{
				NamespaceTemplates: []*string{aws.String("old-ref-ns")},
			},
		}},
	}
	status := []*svcapitypes.MemoryStrategy{
		{Name: aws.String("ep1"), StrategyID: aws.String("id-1")},
	}

	result := buildModifyMemoryStrategies(desired, latest, status)

	if result == nil {
		t.Fatal("expected non-nil result for changed reflection config")
	}
	if len(result.ModifyMemoryStrategies) != 1 {
		t.Fatalf("expected 1 modify, got %d", len(result.ModifyMemoryStrategies))
	}
	mod := result.ModifyMemoryStrategies[0]
	if mod.Configuration == nil {
		t.Error("expected configuration to be set on modify for reflection change")
	}
}

func TestBuildModifyMemoryStrategies_UnchangedStrategiesExcluded(t *testing.T) {
	// 3 strategies, only one changed — only that one should appear in modifies
	desired := []*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:        aws.String("ep1"),
			Description: aws.String("same"),
		}},
		{SemanticMemoryStrategy: &svcapitypes.SemanticMemoryStrategyInput{
			Name:        aws.String("sem1"),
			Description: aws.String("updated"),
		}},
		{SummaryMemoryStrategy: &svcapitypes.SummaryMemoryStrategyInput{
			Name:        aws.String("sum1"),
			Description: aws.String("same"),
		}},
	}
	latest := []*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:        aws.String("ep1"),
			Description: aws.String("same"),
		}},
		{SemanticMemoryStrategy: &svcapitypes.SemanticMemoryStrategyInput{
			Name:        aws.String("sem1"),
			Description: aws.String("old"),
		}},
		{SummaryMemoryStrategy: &svcapitypes.SummaryMemoryStrategyInput{
			Name:        aws.String("sum1"),
			Description: aws.String("same"),
		}},
	}
	status := []*svcapitypes.MemoryStrategy{
		{Name: aws.String("ep1"), StrategyID: aws.String("id-1")},
		{Name: aws.String("sem1"), StrategyID: aws.String("id-2")},
		{Name: aws.String("sum1"), StrategyID: aws.String("id-3")},
	}

	result := buildModifyMemoryStrategies(desired, latest, status)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.AddMemoryStrategies) != 0 {
		t.Errorf("expected 0 adds, got %d", len(result.AddMemoryStrategies))
	}
	if len(result.DeleteMemoryStrategies) != 0 {
		t.Errorf("expected 0 deletes, got %d", len(result.DeleteMemoryStrategies))
	}
	if len(result.ModifyMemoryStrategies) != 1 {
		t.Fatalf("expected 1 modify, got %d", len(result.ModifyMemoryStrategies))
	}
	if *result.ModifyMemoryStrategies[0].MemoryStrategyId != "id-2" {
		t.Errorf("expected modify for id-2, got %s", *result.ModifyMemoryStrategies[0].MemoryStrategyId)
	}
}
