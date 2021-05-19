package sealing

import "github.com/onflow/flow-go/model/flow"

type MatchingCore interface {
	// ProcessReceipt processes a new execution receipt in blocking way.
	// Returns:
	// * exception in case of unexpected error
	// * nil - successfully processed receipt
	ProcessReceipt(originID flow.Identifier, receipt *flow.ExecutionReceipt) error
	// ProcessFinalizedBlock processes finalization events in blocking way.
	// Returns:
	// * exception in case of unexpected error
	// * nil - successfully processed finalized block
	ProcessFinalizedBlock(finalizedBlockID flow.Identifier) error
}
