package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// Payloads represents persistent storage for payloads.
type Payloads interface {

	// Store will store a payload and index its contents.
	Store(blockID flow.Identifier, payload *flow.Payload) error

	// ByBlockID returns the payload with the given hash. It is available for
	// finalized and ambiguous blocks.
	ByBlockID(blockID flow.Identifier) (*flow.Payload, error)
}
