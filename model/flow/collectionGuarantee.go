package flow

import (
	"github.com/onflow/crypto"
)

// CollectionGuarantee is a signed hash for a collection, which is used
// to announce collections to consensus nodes.
//
//structwrite:immutable - mutations allowed only within the constructor
type CollectionGuarantee struct {
	CollectionID     Identifier       // ID of the collection being guaranteed
	ReferenceBlockID Identifier       // defines expiry of the collection
	ChainID          ChainID          // the chainID of the cluster in order to determine which cluster this guarantee belongs to
	SignerIndices    []byte           // encoded indices of the signers
	Signature        crypto.Signature // guarantor signatures
}

func NewCollectionGuarantee(
	collectionID Identifier,
	referenceBlockID Identifier,
	chainID ChainID,
	signerIndices []byte,
	signature crypto.Signature) CollectionGuarantee {
	return CollectionGuarantee{
		CollectionID:     collectionID,
		ReferenceBlockID: referenceBlockID,
		ChainID:          chainID,
		SignerIndices:    signerIndices,
		Signature:        signature,
	}
}

// ID returns the fingerprint of the collection guarantee.
func (cg *CollectionGuarantee) ID() Identifier {
	return cg.CollectionID
}

// Checksum returns a checksum of the collection guarantee including the
// signatures.
func (cg *CollectionGuarantee) Checksum() Identifier {
	return MakeID(cg)
}
