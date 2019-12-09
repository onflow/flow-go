// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package collection

import (
	"github.com/dapperlabs/flow-go/crypto"
)

// GuaranteedCollection represents a signed hash for a collection, which is used
// to announce collections to consensus nodes.
type GuaranteedCollection struct {
	Hash       crypto.Hash
	Signatures []crypto.Signature
}
