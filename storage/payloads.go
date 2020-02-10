// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Payloads represents persistent storage for payloads.
type Payloads interface {

	// Store will store a payload and index it.
	Store(payload *flow.Payload) error

	// ByPayloadHash returns the payload with the given hash. It is available for
	// finalized and ambiguous blocks.
	ByPayloadHash(payloadHash flow.Identifier) (*flow.Payload, error)
}
