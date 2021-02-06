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

	// ByPreviousResultID returns the pending receipt whose previous result id
	// matches the given result id
	ByPreviousResultID(previousReusltID flow.Identifier) (*flow.ExecutionReceipt, bool)
}
