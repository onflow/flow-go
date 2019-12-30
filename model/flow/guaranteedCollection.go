// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"github.com/dapperlabs/flow-go/crypto"
)

// GuaranteedCollection represents a signed hash for a collection, which is used
// to announce collections to consensus nodes.
type GuaranteedCollection struct {
	CollectionFingerprint Fingerprint
	Signatures            []crypto.Signature
}

// TODO we need to fix this later
// Fingerprint returns the fingerprint of the Guaranteed collection.
func (gc *GuaranteedCollection) Fingerprint() Fingerprint {
	return gc.CollectionFingerprint
}

// Hash returns the hash of the collection.
func (gc *GuaranteedCollection) Hash() crypto.Hash {
	return crypto.Hash(gc.CollectionFingerprint)
}
