package flow

// SyncRequest is part of the synchronization protocol and represents a node on
// the network sharing the height of its latest finalized block and requesting
// the same information from the recipient.
type SyncRequest struct {
	Nonce  uint64
	Height uint64
}

// SyncResponse is part of the synchronization protocol and represents the reply
// to a synchronization request that contains the latest finalized block height
// of the responding node.
type SyncResponse struct {
	Nonce  uint64
	Height uint64
}
