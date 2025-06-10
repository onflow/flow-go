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

// UntrustedCollectionGuarantee is an untrusted input-only representation of an CollectionGuarantee,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedCollectionGuarantee should be validated and converted into
// a trusted CollectionGuarantee using NewCollectionGuarantee constructor.
type UntrustedCollectionGuarantee CollectionGuarantee

// NewCollectionGuarantee creates a new instance of CollectionGuarantee.
// Construction CollectionGuarantee allowed only within the constructor
func NewCollectionGuarantee(untrustedGuarantee UntrustedCollectionGuarantee) *CollectionGuarantee {
	return &CollectionGuarantee{
		CollectionID:     untrustedGuarantee.CollectionID,
		ReferenceBlockID: untrustedGuarantee.ReferenceBlockID,
		ChainID:          untrustedGuarantee.ChainID,
		SignerIndices:    untrustedGuarantee.SignerIndices,
		Signature:        untrustedGuarantee.Signature,
	}
}

// ID returns a collision-resistant hash of the CollectionGuarantee struct.
// This is distinct from the ID of the corresponding collection.
func (cg *CollectionGuarantee) ID() Identifier {
	return MakeID(cg)
}
