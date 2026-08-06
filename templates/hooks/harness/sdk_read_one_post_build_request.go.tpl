	// Omitting HarnessVersion makes GetHarness return the current mutable
	// configuration. Passing the last observed version would pin this read to an
	// immutable historical version and hide out-of-band updates from ACK.
	prepareGetHarnessInput(input)
