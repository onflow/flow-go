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

	// ByBlockID retrieves the seals in the payload of a block.
	ByBlockID(blockID flow.Identifier) ([]*flow.Seal, error)

	// BySealedID retrieves the seal for a block that was sealed.
	BySealedID(sealID flow.Identifier) (*flow.Seal, error)
}
