	if delta.DifferentAt("Spec.MemoryStrategies") {
		input.MemoryStrategies = buildModifyMemoryStrategies(
			desired.ko.Spec.MemoryStrategies,
			latest.ko.Status.Strategies,
		)
	}
