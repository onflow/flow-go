package storage

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// ProposalSignatures represents persistent storage for proposer signatures on block proposals.
type ProposalSignatures interface {
	// StoreTx stores a proposal signature as part of a database transaction, indexed by the associated BlockID.
	// Error returns:
	//  - storage.ErrAlreadyExists if any proposal signature for the given blockID is already present
	StoreTx(blockID flow.Identifier, sig []byte) func(*transaction.Tx) error

	// ByBlockID returns the proposer signature for the given blockID. It is available for finalized and ambiguous blocks.
	// Error returns:
	//  - ErrNotFound if no block header with the given ID exists
	ByBlockID(blockID flow.Identifier) ([]byte, error)
}
