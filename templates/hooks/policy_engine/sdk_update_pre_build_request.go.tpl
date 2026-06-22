	if delta.DifferentAt("Spec.Tags") {
		if err := rm.syncTags(ctx, desired, latest); err != nil {
			// TagResource/UntagResource may not be supported for PolicyEngine
			// resources. Log the error but do not fail the update.
			rlog.Info("unable to sync tags for resource, skipping", "error", err)
		}
	}
	if !delta.DifferentExcept("Spec.Tags") {
		return desired, nil
	}
