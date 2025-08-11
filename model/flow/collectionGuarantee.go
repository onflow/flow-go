package flow

import (
	"fmt"

	"github.com/onflow/crypto"
)

// CollectionGuarantee is a signed hash for a collection, which is used
// to announce collections to consensus nodes.
//
//structwrite:immutable - mutations allowed only within the constructor
type CollectionGuarantee struct {
	CollectionID     Identifier       // ID of the collection being guaranteed
	ReferenceBlockID Identifier       // defines expiry of the collection
	ClusterChainID   ChainID          // the chainID of the cluster in order to determine which cluster this guarantee belongs to
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
// Construction of CollectionGuarantee is allowed only within the constructor.
//
// This constructor enforces basic structural validity, ensuring critical fields like
// CollectionID and ReferenceBlockID are non-zero.
// The Signature field is not validated here for the following reasons:
//
//   - Signature is currently unused and set to nil when generating a CollectionGuarantee,
//     as the consensus nodes are currently unable to easily verify it.
func NewCollectionGuarantee(untrusted UntrustedCollectionGuarantee) (*CollectionGuarantee, error) {
	if untrusted.CollectionID == ZeroID {
		return nil, fmt.Errorf("CollectionID must not be empty")
	}

	if untrusted.ReferenceBlockID == ZeroID {
		return nil, fmt.Errorf("ReferenceBlockID must not be empty")
	}

	if len(untrusted.SignerIndices) == 0 {
		return nil, fmt.Errorf("SignerIndices must not be empty")
	}

	if len(untrusted.ClusterChainID) == 0 {
		return nil, fmt.Errorf("ClusterChainID must not be empty")
	}

	return &CollectionGuarantee{
		CollectionID:     untrusted.CollectionID,
		ReferenceBlockID: untrusted.ReferenceBlockID,
		ClusterChainID:   untrusted.ClusterChainID,
		SignerIndices:    untrusted.SignerIndices,
		Signature:        untrusted.Signature,
	}, nil
}

// ID returns a collision-resistant hash of the CollectionGuarantee struct.
// This is distinct from the ID of the corresponding collection.
func (cg *CollectionGuarantee) ID() Identifier {
	return MakeID(cg)
}
