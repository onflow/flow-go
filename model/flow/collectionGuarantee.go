// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"github.com/dapperlabs/flow-go/crypto"
)

// CollectionGuarantee is a signed hash for a collection, which is used
// to announce collections to consensus nodes.
type CollectionGuarantee struct {
	CollectionHash crypto.Hash
	Signatures     []crypto.Signature
}

// TODO we need to fix this later
// Fingerprint returns the fingerprint of the collection guarantee.
func (g *CollectionGuarantee) Fingerprint() Fingerprint {
	return Fingerprint(g.CollectionHash)
}

// Hash returns the hash of the collection that is guaranteed.
func (g *CollectionGuarantee) Hash() crypto.Hash {
	return g.CollectionHash
}
