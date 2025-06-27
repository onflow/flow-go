package flow

import (
	"fmt"

	"github.com/onflow/crypto"
)

// CollectionGuarantee is a signed hash for a collection, which is used
// to announce collections to consensus nodes.
//
type CollectionGuarantee struct {
	CollectionID     Identifier       // ID of the collection being guaranteed
	ReferenceBlockID Identifier       // defines expiry of the collection
	ChainID          ChainID          // the chainID of the cluster in order to determine which cluster this guarantee belongs to
	SignerIndices    []byte           // encoded indices of the signers
	Signature        crypto.Signature // guarantor signatures
}

// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
type UntrustedCollectionGuarantee CollectionGuarantee

//
// All errors indicate a valid CollectionGuarantee cannot be constructed from the input.
func NewCollectionGuarantee(untrusted UntrustedCollectionGuarantee) (*CollectionGuarantee, error) {
	if untrusted.CollectionID == ZeroID {
		return nil, fmt.Errorf("collection ID must not be zero")
	}
	if untrusted.ReferenceBlockID == ZeroID {
		return nil, fmt.Errorf("reference block ID must not be zero")
	}
	if len(untrusted.Signature) == 0 {
		return nil, fmt.Errorf("signature must not be empty")
	}
	return &CollectionGuarantee{
		CollectionID:     untrusted.CollectionID,
		ReferenceBlockID: untrusted.ReferenceBlockID,
		ChainID:          untrusted.ChainID,
		SignerIndices:    untrusted.SignerIndices,
		Signature:        untrusted.Signature,
	}, nil
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
