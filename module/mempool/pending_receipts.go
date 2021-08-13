package mempool

import "github.com/onflow/flow-go/model/flow"

// PendingReceipts stores pending receipts indexed by the id.
// It also maintains a secondary index on the previous result id, which is unique,
// in order to allow to find a receipt by the previous result id.
type PendingReceipts interface {
	// Add a pending receipt
	// return true if added
	// return false if is a duplication
	Add(receipt *flow.ExecutionReceipt) bool

	// Remove a pending receipt by ID
	Rem(receiptID flow.Identifier) bool

	// ByPreviousResultID returns all the pending receipts whose previous result id
	// matches the given result id
	ByPreviousResultID(previousReusltID flow.Identifier) []*flow.ExecutionReceipt

	// PruneUpToHeight remove all receipts for blocks whose height is strictly
	// smaller that height. Note: receipts for blocks at height are retained.
	// After pruning, receipts below for blocks below the given height are dropped.
	//
	// Monotonicity Requirement:
	// The pruned height cannot decrease, as we cannot recover already pruned elements.
	// If `height` is smaller than the previous value, the previous value is kept
	// and the sentinel mempool.DecreasingPruningHeightError is returned.
	PruneUpToHeight(height uint64) error
}
