// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package collection

import (
	"github.com/dapperlabs/flow-go/crypto"
)

// GuaranteedCollection represents a signed hash for a collection, which is used
// to announce collections to consensus nodes.
type GuaranteedCollection struct {
	CollectionHash crypto.Hash
	Signatures     []crypto.Signature
}

// Hash returns the hash of the collection.
func (gc *GuaranteedCollection) Hash() crypto.Hash {
	return gc.CollectionHash
}
