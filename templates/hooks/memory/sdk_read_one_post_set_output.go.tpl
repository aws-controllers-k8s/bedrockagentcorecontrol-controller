	if !memorySettled(&resource{ko}) {
		return nil, requeueNotReady
	}
	tags, err := rm.getTags(ctx, string(*ko.Status.ACKResourceMetadata.ARN))
	if err != nil {
		return nil, err
	}
	ko.Spec.Tags = tags