package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// ProposalSignatures represents persistent storage for proposer signatures on block proposals.
// The proposer's signature is only transiently important conceptually (until the block obtains a QC),
// but our current business logic validates it even in cases where it is not strictly necessary.
// For simplicity, we require it be stored for all blocks, however it is stored separately to
// make it easier to remove in the future if/when we update the syncing and block ingestion logic.
type ProposalSignatures interface {
	// ByBlockID returns the proposer signature for the given blockID. It is available for finalized and ambiguous blocks.
	// Error returns:
	//  - ErrNotFound if no block header with the given ID exists
	ByBlockID(blockID flow.Identifier) ([]byte, error)
}
