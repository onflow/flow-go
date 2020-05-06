// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Payloads represents persistent storage for payloads.
type Payloads interface {

	// Store will store a payload and index its contents.
	Store(blockID flow.Identifier, payload *flow.Payload) error

	// ByBlockID returns the payload with the given hash. It is available for
	// finalized and ambiguous blocks.
	ByBlockID(blockID flow.Identifier) (*flow.Payload, error)

	// IdentitiesFor will return the identities for a black.
	IdentitiesFor(blockID flow.Identifier) (flow.IdentityList, error)

	// GuaranteesFor will return the for a block.
	GuaranteesFor(blockID flow.Identifier) ([]*flow.CollectionGuarantee, error)

	// SealsFor will return the seals for a block.
	SealsFor(blockID flow.Identifier) ([]*flow.Seal, error)
}
