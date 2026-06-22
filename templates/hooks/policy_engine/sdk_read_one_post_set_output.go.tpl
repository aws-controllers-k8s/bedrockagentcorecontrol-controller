	tags, err := rm.getTags(ctx, string(*ko.Status.ACKResourceMetadata.ARN))
	if err != nil {
		// ListTagsForResource may not be supported for PolicyEngine resources.
		// Log the error but do not fail the read — the resource should still
		// reconcile even if tag fetching is unavailable.
		rlog.Info("unable to retrieve tags for resource, skipping", "error", err)
		err = nil
	} else {
		ko.Spec.Tags = tags
	}
