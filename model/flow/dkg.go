package flow

// DKGEndState captures the final state of a completed DKG.
type DKGEndState uint32

const (
	// DKGEndStateUnknown - zero value for this enum, indicates unset value
	DKGEndStateUnknown DKGEndState = iota
	// DKGEndStateSuccess - the DKG completed, this node has a valid beacon key.
	DKGEndStateSuccess
	// DKGEndStateInconsistentKey - the DKG completed, this node has an invalid beacon key.
	DKGEndStateInconsistentKey
	// DKGEndStateNoKey - this node did not store a key, typically caused by a crash mid-DKG.
	DKGEndStateNoKey
	// DKGEndStateDKGFailure - the underlying DKG library reported an error.
	DKGEndStateDKGFailure
)

func (state DKGEndState) String() string {
	switch state {
	case DKGEndStateSuccess:
		return "DKGEndStateSuccess"
	case DKGEndStateInconsistentKey:
		return "DKGEndStateInconsistentKey"
	case DKGEndStateNoKey:
		return "DKGEndStateNoKey"
	case DKGEndStateDKGFailure:
		return "DKGEndStateDKGFailure"
	default:
		return "DKGEndStateUnknown"
	}
}
