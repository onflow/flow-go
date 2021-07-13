package consensus

import "github.com/onflow/flow-go/model/flow"

// MatchingCore collects inbound receipts from Execution Node
// for potential inclusion in future blocks.
// Implementations of MatchingCore are generally NOT concurrency safe.
type MatchingCore interface {
	// ProcessReceipt processes a new execution receipt in blocking way.
	// Returns:
	// * exception in case of unexpected error
	// * nil - successfully processed receipt
	ProcessReceipt(receipt *flow.ExecutionReceipt) error
	// OnBlockFinalization notifies the engine that a (potentially new) block was finalized.
	// Implementations are non-blocking.
	// Returns:
	// * exception in case of unexpected error
	// * nil - successfully processed finalized block
	OnBlockFinalization() error
}
