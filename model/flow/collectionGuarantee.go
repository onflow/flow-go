// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"github.com/dapperlabs/flow-go/crypto"
)

// CollectionGuarantee is a signed hash for a collection, which is used
// to announce collections to consensus nodes.
type CollectionGuarantee struct {
	// Hash is the hash of collection that is being guaranteed.
	CollectionID Identifier
	Signatures   []crypto.Signature
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
