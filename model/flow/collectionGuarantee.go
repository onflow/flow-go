package flow

import (
	"github.com/onflow/crypto"
)

// CollectionGuarantee is a signed hash for a collection, which is used
// to announce collections to consensus nodes.
type CollectionGuarantee struct {
	CollectionID     Identifier       // ID of the collection being guaranteed
	ReferenceBlockID Identifier       // defines expiry of the collection
	ChainID          ChainID          // the chainID of the cluster in order to determine which cluster this guarantee belongs to
	SignerIndices    []byte           // encoded indices of the signers
	Signature        crypto.Signature // guarantor signatures
}

// ID returns a collision-resistant hash of the CollectionGuarantee struct.
// This is distinct from the ID of the corresponding collection.
func (cg *CollectionGuarantee) ID() Identifier {
	return cg.CollectionID
}
