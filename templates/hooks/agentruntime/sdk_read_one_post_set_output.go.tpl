	if !agentRuntimeReady(&resource{ko}) {
		return nil, requeueNotReady
	}