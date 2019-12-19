// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package collection

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// GuaranteedCollection represents a signed hash for a collection, which is used
// to announce collections to consensus nodes.
type GuaranteedCollection struct {
	CollectionHash crypto.Hash
	Signatures     []crypto.Signature
}

// TODO we need to fix this later
// Fingerprint returns the fingerprint of the Guaranteed collection.
func (gc *GuaranteedCollection) Fingerprint() flow.Fingerprint {
	return gc.CollectionHash
}

// Hash returns the hash of the collection.
func (gc *GuaranteedCollection) Hash() crypto.Hash {
	return gc.CollectionHash
}
