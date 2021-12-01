package flow

// EndState captures the final state of a completed DKG.
type EndState uint32

const (
	// EndStateUnknown - zero value for this enum, indicates unset value
	EndStateUnknown EndState = iota
	// EndStateSuccess - the DKG completed, this node has a valid beacon key.
	EndStateSuccess
	// EndStateInconsistentKey - the DKG completed, this node has an invalid beacon key.
	EndStateInconsistentKey
	// EndStateNoKey - this node did not store a key, typically caused by a crash mid-DKG.
	EndStateNoKey
	// EndStateDKGFailure - the underlying DKG library reported an error.
	EndStateDKGFailure
)
