// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"github.com/dapperlabs/flow-go/crypto"
)

// CollectionGuarantee is a signed hash for a collection, which is used
// to announce collections to consensus nodes.
type CollectionGuarantee struct {
	CollectionID     Identifier       // ID of the collection being guaranteed
	ReferenceBlockID Identifier       // defines expiry of the collection
	SignerIDs        []Identifier     // list of guarantors
	Signature        crypto.Signature // guarantor signatures
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
