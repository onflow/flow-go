// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Seals represents persistent storage for seals.
type Seals interface {

	// Store inserts the seal.
	Store(guarantee *flow.Seal) error

	// ByID retrieves the seal by the collection
	// fingerprint.
	ByID(sealID flow.Identifier) (*flow.Seal, error)

	// ByBlockID retrieves the last seal in the chain of seals for the block.
	ByBlockID(sealedID flow.Identifier) (*flow.Seal, error)
}
