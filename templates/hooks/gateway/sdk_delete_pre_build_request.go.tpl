	if !gatewaySettled(r) {
		return nil, requeueNotReady
	}
