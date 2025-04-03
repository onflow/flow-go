package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// ProposalSignatures represents persistent storage for proposer signatures on block proposals.
type ProposalSignatures interface {
	// ByBlockID returns the proposer signature for the given blockID. It is available for finalized and ambiguous blocks.
	// Error returns:
	//  - ErrNotFound if no block header with the given ID exists
	ByBlockID(blockID flow.Identifier) ([]byte, error)
}
