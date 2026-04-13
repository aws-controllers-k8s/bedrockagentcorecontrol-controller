	if !memorySettled(r) {
		return nil, requeueNotReady
	}