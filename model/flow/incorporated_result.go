package flow

// IncorporatedResult is a wrapper around an ExecutionResult which contains the
// ID of the first block on its fork in which it was incorporated.
type IncorporatedResult struct {
	// IncorporatedBlockID is the ID of the first block on its fork where a
	// receipt for this result was incorporated. Within a fork, multiple blocks
	// may contain receipts for the same result; only the first one is used to
	// compute the random beacon of the result's chunk assignment. Tracking
	// results by incorporation block is also instrumental in making the sealing
	// logic fork-aware.
	IncorporatedBlockID Identifier

	// Result is the ExecutionResult contained in the ExecutionReceipt that was
	// incorporated in the payload of IncorporatedBlockID.
	Result *ExecutionResult
}

// ID implements flow.Entity.ID for IncorporatedResult to make it capable of
// being stored directly in mempools and storage.
func (ir *IncorporatedResult) ID() Identifier {
	return MakeID(ir)
}

// CheckSum implements flow.Entity.CheckSum for IncorporatedResult to make it
// capable of being stored directly in mempools and storage.
func (ir *IncorporatedResult) Checksum() Identifier {
	return MakeID(ir)
}
