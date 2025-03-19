package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

// InsertProposalSignature inserts a quorum certificate by block ID.
// Returns storage.ErrAlreadyExists if a proposal signature has already been inserted for the block.
func InsertProposalSignature(blockID flow.Identifier, sig *[]byte) func(*badger.Txn) error {
	return insert(makePrefix(codeBlockIDToProposalSignature, blockID), sig)
}

// RetrieveProposalSignature retrieves a quorum certificate by blockID.
// Returns storage.ErrNotFound if no proposal signature is stored for the block.
func RetrieveProposalSignature(blockID flow.Identifier, sig *[]byte) func(*badger.Txn) error {
	return retrieve(makePrefix(codeBlockIDToProposalSignature, blockID), sig)
}
