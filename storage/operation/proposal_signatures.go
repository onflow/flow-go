package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertProposalSignature inserts a proposal signature by block ID.
// Returns storage.ErrAlreadyExists if a proposal signature has already been inserted for the block.
func InsertProposalSignature(w storage.Writer, blockID flow.Identifier, sig *[]byte) error {
	return UpsertByKey(w, MakePrefix(codeBlockIDToProposalSignature, blockID), sig)
}

// RetrieveProposalSignature retrieves a proposal signature by blockID.
// Returns storage.ErrNotFound if no proposal signature is stored for the block.
func RetrieveProposalSignature(r storage.Reader, blockID flow.Identifier, sig *[]byte) error {
	return RetrieveByKey(r, MakePrefix(codeBlockIDToProposalSignature, blockID), sig)
}
