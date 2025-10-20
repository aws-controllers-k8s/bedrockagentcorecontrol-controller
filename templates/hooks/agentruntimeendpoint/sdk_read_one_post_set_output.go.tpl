	if !agentRuntimeEndpointReady(&resource{ko}) {
		return nil, requeueNotReady
	}