package hotstuff

// Mempool defines how the HotStuff core algorithm interacts with a mempool of
// pending objects awaiting consensus.
//
// For example, in consensus nodes, these objects are collection guarantees,
// in collection nodes, these objects are transactions.
type Mempool interface {

	// CreatePayload generates a payload for a new block for the given view.
	// given block ID and returns the hash of this payload. The payload must be
	// consistent with the chain it is extending (ie. payload contents should
	// not conflict).
	CreatePayload(view uint64) (hash []byte)

	// Finalized marks a payload as having been included in a finalized block.
	//
	// This means that the payload can be released. Any contents of the payload
	// that exist either in the mempool or in another pending payload should be
	// removed.
	Finalized(payload []byte)

	// Pruned marks a payload as having been included in a block that was
	// never finalized.
	//
	// This means that the payload contents should be added back to the mempool.
	Pruned(payload []byte)
}

// NOTE: The engine side of the mempool must be responsible for telling the
// mempool about payloads that exist in blocks besides those that the HotStuff
// core creates on its own.
