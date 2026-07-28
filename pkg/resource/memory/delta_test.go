package memory

import (
	"testing"

	svcapitypes "github.com/aws-controllers-k8s/bedrockagentcorecontrol-controller/apis/v1alpha1"
	"github.com/aws/aws-sdk-go-v2/aws"
)

func makeMemoryResource(strategies []*svcapitypes.MemoryStrategyInput) *resource {
	return &resource{
		ko: &svcapitypes.Memory{
			Spec: svcapitypes.MemorySpec{
				MemoryStrategies: strategies,
			},
		},
	}
}

func hasDelta(a, b *resource) bool {
	delta := newResourceDelta(a, b)
	return delta.DifferentAt("Spec.MemoryStrategies")
}

func TestCompareMemoryStrategies_BothNil(t *testing.T) {
	a := makeMemoryResource(nil)
	b := makeMemoryResource(nil)

	if hasDelta(a, b) {
		t.Error("expected no difference")
	}
}

func TestCompareMemoryStrategies_BothEmpty(t *testing.T) {
	a := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{})
	b := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{})

	if hasDelta(a, b) {
		t.Error("expected no difference")
	}
}

func TestCompareMemoryStrategies_DifferentLength(t *testing.T) {
	a := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{Name: aws.String("ep1")}},
	})
	b := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{Name: aws.String("ep1")}},
		{SemanticMemoryStrategy: &svcapitypes.SemanticMemoryStrategyInput{Name: aws.String("sem1")}},
	})

	if !hasDelta(a, b) {
		t.Error("expected a difference for different lengths")
	}
}

func TestCompareMemoryStrategies_SameOrderIdentical(t *testing.T) {
	strats := func() []*svcapitypes.MemoryStrategyInput {
		return []*svcapitypes.MemoryStrategyInput{
			{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
				Name:        aws.String("ep1"),
				Description: aws.String("desc"),
			}},
			{SemanticMemoryStrategy: &svcapitypes.SemanticMemoryStrategyInput{
				Name: aws.String("sem1"),
			}},
		}
	}
	a := makeMemoryResource(strats())
	b := makeMemoryResource(strats())

	if hasDelta(a, b) {
		t.Error("expected no difference")
	}
}

func TestCompareMemoryStrategies_DifferentOrder_NoDelta(t *testing.T) {
	a := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{Name: aws.String("ep1")}},
		{SemanticMemoryStrategy: &svcapitypes.SemanticMemoryStrategyInput{Name: aws.String("sem1")}},
	})
	b := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{SemanticMemoryStrategy: &svcapitypes.SemanticMemoryStrategyInput{Name: aws.String("sem1")}},
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{Name: aws.String("ep1")}},
	})

	if hasDelta(a, b) {
		t.Error("expected no difference when order differs")
	}
}

func TestCompareMemoryStrategies_DefaultNamespaces_NoDelta(t *testing.T) {
	a := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name: aws.String("ep1"),
		}},
	})
	b := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:               aws.String("ep1"),
			Namespaces:         []*string{aws.String("default")},
			NamespaceTemplates: []*string{aws.String("default")},
		}},
	})

	if hasDelta(a, b) {
		t.Error("expected no difference when latest has default namespaces")
	}
}

func TestCompareMemoryStrategies_DefaultNamespaces_Semantic_NoDelta(t *testing.T) {
	a := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{SemanticMemoryStrategy: &svcapitypes.SemanticMemoryStrategyInput{
			Name: aws.String("sem1"),
		}},
	})
	b := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{SemanticMemoryStrategy: &svcapitypes.SemanticMemoryStrategyInput{
			Name:               aws.String("sem1"),
			Namespaces:         []*string{aws.String("default")},
			NamespaceTemplates: []*string{aws.String("default")},
		}},
	})

	if hasDelta(a, b) {
		t.Error("expected no difference when latest has default namespaces")
	}
}

func TestCompareMemoryStrategies_DefaultNamespaces_Custom_NoDelta(t *testing.T) {
	a := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{CustomMemoryStrategy: &svcapitypes.CustomMemoryStrategyInput{
			Name: aws.String("custom1"),
		}},
	})
	b := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{CustomMemoryStrategy: &svcapitypes.CustomMemoryStrategyInput{
			Name:               aws.String("custom1"),
			Namespaces:         []*string{aws.String("default")},
			NamespaceTemplates: []*string{aws.String("default")},
		}},
	})

	if hasDelta(a, b) {
		t.Error("expected no difference when latest has default namespaces")
	}
}

func TestCompareMemoryStrategies_UnsetNamespaces_ServerPopulated_NoDelta(t *testing.T) {
	// User leaves namespaces unset; the service populates a value. Whatever the
	// service assigns to an unset field is a server-side default and must be
	// adopted without diffing — this holds for the new templated defaults, for
	// which "custom-ns" here stands in.
	a := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name: aws.String("ep1"),
		}},
	})
	b := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:       aws.String("ep1"),
			Namespaces: []*string{aws.String("custom-ns")},
		}},
	})

	if hasDelta(a, b) {
		t.Error("expected no difference when the service populates an unset namespaces field")
	}
}

func TestCompareMemoryStrategies_TemplatedDefault_NoDelta(t *testing.T) {
	// The concrete regression: user leaves namespaceTemplates unset and the
	// service returns the current templated default. This must not diff.
	a := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{SemanticMemoryStrategy: &svcapitypes.SemanticMemoryStrategyInput{
			Name: aws.String("sem1"),
		}},
	})
	b := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{SemanticMemoryStrategy: &svcapitypes.SemanticMemoryStrategyInput{
			Name:               aws.String("sem1"),
			Namespaces:         []*string{aws.String("/strategies/{memoryStrategyId}/actors/{actorId}/")},
			NamespaceTemplates: []*string{aws.String("/strategies/{memoryStrategyId}/actors/{actorId}/")},
		}},
	})

	if hasDelta(a, b) {
		t.Error("expected no difference when the service returns templated defaults for unset fields")
	}
}

func TestCompareMemoryStrategies_ExplicitNamespaces_Match(t *testing.T) {
	a := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:       aws.String("ep1"),
			Namespaces: []*string{aws.String("my-ns")},
		}},
	})
	b := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:       aws.String("ep1"),
			Namespaces: []*string{aws.String("my-ns")},
		}},
	})

	if hasDelta(a, b) {
		t.Error("expected no difference")
	}
}

func TestCompareMemoryStrategies_DescriptionChanged_HasDelta(t *testing.T) {
	a := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:        aws.String("ep1"),
			Description: aws.String("old desc"),
		}},
	})
	b := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:        aws.String("ep1"),
			Description: aws.String("new desc"),
		}},
	})

	if !hasDelta(a, b) {
		t.Error("expected a difference for changed description")
	}
}

func TestCompareMemoryStrategies_DifferentOrder_WithDefaults_NoDelta(t *testing.T) {
	a := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{Name: aws.String("ep1")}},
		{SummaryMemoryStrategy: &svcapitypes.SummaryMemoryStrategyInput{Name: aws.String("sum1")}},
		{SemanticMemoryStrategy: &svcapitypes.SemanticMemoryStrategyInput{Name: aws.String("sem1")}},
	})
	b := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{SemanticMemoryStrategy: &svcapitypes.SemanticMemoryStrategyInput{
			Name:               aws.String("sem1"),
			Namespaces:         []*string{aws.String("default")},
			NamespaceTemplates: []*string{aws.String("default")},
		}},
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:               aws.String("ep1"),
			Namespaces:         []*string{aws.String("default")},
			NamespaceTemplates: []*string{aws.String("default")},
		}},
		{SummaryMemoryStrategy: &svcapitypes.SummaryMemoryStrategyInput{
			Name:               aws.String("sum1"),
			Namespaces:         []*string{aws.String("default")},
			NamespaceTemplates: []*string{aws.String("default")},
		}},
	})

	if hasDelta(a, b) {
		t.Error("expected no difference with reordered + default namespaces")
	}
}

func TestCompareMemoryStrategies_UserExplicitlySetDefault_HasDelta(t *testing.T) {
	// User explicitly sets ["default"] in desired, latest has nil — this IS a real diff
	a := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:       aws.String("ep1"),
			Namespaces: []*string{aws.String("default")},
		}},
	})
	b := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name: aws.String("ep1"),
		}},
	})

	if !hasDelta(a, b) {
		t.Error("expected a difference when user explicitly sets default but latest is nil")
	}
}

func TestCompareMemoryStrategies_UserExplicitlySetDefault_LatestAlsoDefault_NoDelta(t *testing.T) {
	// User explicitly sets ["default"], AWS also returns ["default"] — no delta
	a := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:       aws.String("ep1"),
			Namespaces: []*string{aws.String("default")},
		}},
	})
	b := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:       aws.String("ep1"),
			Namespaces: []*string{aws.String("default")},
		}},
	})

	if hasDelta(a, b) {
		t.Error("expected no difference when both sides have default")
	}
}

func TestCompareMemoryStrategies_MirroredNamespaceTemplates_NoDelta(t *testing.T) {
	// User sets namespaceTemplates only, AWS mirrors it to namespaces
	a := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:               aws.String("ep1"),
			NamespaceTemplates: []*string{aws.String("my-ns")},
		}},
	})
	b := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:               aws.String("ep1"),
			Namespaces:         []*string{aws.String("my-ns")},
			NamespaceTemplates: []*string{aws.String("my-ns")},
		}},
	})

	if hasDelta(a, b) {
		t.Error("expected no difference when AWS mirrors namespaceTemplates to namespaces")
	}
}

func TestCompareMemoryStrategies_MirroredNamespaces_NoDelta(t *testing.T) {
	// User sets namespaces only, AWS mirrors it to namespaceTemplates
	a := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{SemanticMemoryStrategy: &svcapitypes.SemanticMemoryStrategyInput{
			Name:       aws.String("sem1"),
			Namespaces: []*string{aws.String("my-ns")},
		}},
	})
	b := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{SemanticMemoryStrategy: &svcapitypes.SemanticMemoryStrategyInput{
			Name:               aws.String("sem1"),
			Namespaces:         []*string{aws.String("my-ns")},
			NamespaceTemplates: []*string{aws.String("my-ns")},
		}},
	})

	if hasDelta(a, b) {
		t.Error("expected no difference when AWS mirrors namespaces to namespaceTemplates")
	}
}

func TestCompareMemoryStrategies_SetFieldDiffers_HasDelta(t *testing.T) {
	// User set namespaceTemplates=["foo"] but the service has ["bar"] for that
	// same set field — a genuine drift the user cares about. The sibling
	// namespaces field is unset in desired, so its server value is adopted; the
	// delta must come solely from the set field disagreeing.
	a := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:               aws.String("ep1"),
			NamespaceTemplates: []*string{aws.String("foo")},
		}},
	})
	b := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:               aws.String("ep1"),
			Namespaces:         []*string{aws.String("server-assigned")},
			NamespaceTemplates: []*string{aws.String("bar")},
		}},
	})

	if !hasDelta(a, b) {
		t.Error("expected a difference when a user-set field drifts")
	}
}

func TestCompareMemoryStrategies_SetNamespacesDrifts_HasDelta(t *testing.T) {
	// The symmetric guarantee to SetFieldDiffers: the user set the plain
	// namespaces field and the server has a different value for it. Option A
	// must NOT strip a field the user set, so this legitimate change diffs.
	a := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{SemanticMemoryStrategy: &svcapitypes.SemanticMemoryStrategyInput{
			Name:       aws.String("sem1"),
			Namespaces: []*string{aws.String("my-ns")},
		}},
	})
	b := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{SemanticMemoryStrategy: &svcapitypes.SemanticMemoryStrategyInput{
			Name:       aws.String("sem1"),
			Namespaces: []*string{aws.String("changed-ns")},
		}},
	})

	if !hasDelta(a, b) {
		t.Error("expected a difference when a user-set namespaces value drifts")
	}
}

func TestCompareMemoryStrategies_SetNamespacesDrifts_ServerMirrors_HasDelta(t *testing.T) {
	// Realistic shape of the drift: the user set only namespaces, and the
	// service mirrors the current (drifted) value into namespaceTemplates. The
	// mirrored, unset-in-desired namespaceTemplates is stripped, so the delta
	// must still surface from the user-set namespaces field disagreeing rather
	// than being masked by the mirror.
	a := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{SemanticMemoryStrategy: &svcapitypes.SemanticMemoryStrategyInput{
			Name:       aws.String("sem1"),
			Namespaces: []*string{aws.String("my-ns")},
		}},
	})
	b := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{SemanticMemoryStrategy: &svcapitypes.SemanticMemoryStrategyInput{
			Name:               aws.String("sem1"),
			Namespaces:         []*string{aws.String("changed-ns")},
			NamespaceTemplates: []*string{aws.String("changed-ns")},
		}},
	})

	if !hasDelta(a, b) {
		t.Error("expected a difference when a user-set namespaces value drifts and the server mirrors it")
	}
}

func TestCompareMemoryStrategies_SameLengthStrategySwap_HasDelta(t *testing.T) {
	// Count is unchanged but a strategy was replaced with a different one.
	// Element-wise comparison after name-sorting must catch this.
	a := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{Name: aws.String("ep1")}},
	})
	b := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{SemanticMemoryStrategy: &svcapitypes.SemanticMemoryStrategyInput{Name: aws.String("sem1")}},
	})

	if !hasDelta(a, b) {
		t.Error("expected a difference when a strategy is swapped for a different one")
	}
}

func TestCompareMemoryStrategies_BothSetExplicitly_NoDelta(t *testing.T) {
	// User sets both namespaces and namespaceTemplates explicitly
	a := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:               aws.String("ep1"),
			Namespaces:         []*string{aws.String("ns1")},
			NamespaceTemplates: []*string{aws.String("nst1")},
		}},
	})
	b := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:               aws.String("ep1"),
			Namespaces:         []*string{aws.String("ns1")},
			NamespaceTemplates: []*string{aws.String("nst1")},
		}},
	})

	if hasDelta(a, b) {
		t.Error("expected no difference when both are set identically")
	}
}

func TestCompareMemoryStrategies_EpisodicReflectionMirrored_NoDelta(t *testing.T) {
	// User sets reflectionConfiguration.namespaceTemplates only,
	// AWS mirrors it to reflectionConfiguration.namespaces
	a := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:               aws.String("ep1"),
			NamespaceTemplates: []*string{aws.String("/strategies/{memoryStrategyId}/actors/{actorId}/")},
			ReflectionConfiguration: &svcapitypes.EpisodicReflectionConfigurationInput{
				NamespaceTemplates: []*string{aws.String("/strategies/{memoryStrategyId}/actors/{actorId}/")},
			},
		}},
	})
	b := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:               aws.String("ep1"),
			Namespaces:         []*string{aws.String("/strategies/{memoryStrategyId}/actors/{actorId}/")},
			NamespaceTemplates: []*string{aws.String("/strategies/{memoryStrategyId}/actors/{actorId}/")},
			ReflectionConfiguration: &svcapitypes.EpisodicReflectionConfigurationInput{
				Namespaces:         []*string{aws.String("/strategies/{memoryStrategyId}/actors/{actorId}/")},
				NamespaceTemplates: []*string{aws.String("/strategies/{memoryStrategyId}/actors/{actorId}/")},
			},
		}},
	})

	if hasDelta(a, b) {
		t.Error("expected no difference when AWS mirrors reflection namespaceTemplates to namespaces")
	}
}

func TestCompareMemoryStrategies_EpisodicReflectionDefault_NoDelta(t *testing.T) {
	// User doesn't set reflectionConfiguration, AWS returns it with defaults
	a := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:               aws.String("ep1"),
			NamespaceTemplates: []*string{aws.String("default")},
		}},
	})
	b := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{EpisodicMemoryStrategy: &svcapitypes.EpisodicMemoryStrategyInput{
			Name:               aws.String("ep1"),
			Namespaces:         []*string{aws.String("default")},
			NamespaceTemplates: []*string{aws.String("default")},
			ReflectionConfiguration: &svcapitypes.EpisodicReflectionConfigurationInput{
				Namespaces:         []*string{aws.String("default")},
				NamespaceTemplates: []*string{aws.String("default")},
			},
		}},
	})

	if hasDelta(a, b) {
		t.Error("expected no difference when AWS adds default reflection config")
	}
}

func TestCompareMemoryStrategies_UserPreference_DefaultNamespaces_NoDelta(t *testing.T) {
	a := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{UserPreferenceMemoryStrategy: &svcapitypes.UserPreferenceMemoryStrategyInput{
			Name: aws.String("up1"),
		}},
	})
	b := makeMemoryResource([]*svcapitypes.MemoryStrategyInput{
		{UserPreferenceMemoryStrategy: &svcapitypes.UserPreferenceMemoryStrategyInput{
			Name:               aws.String("up1"),
			Namespaces:         []*string{aws.String("default")},
			NamespaceTemplates: []*string{aws.String("default")},
		}},
	})

	if hasDelta(a, b) {
		t.Error("expected no difference")
	}
}
