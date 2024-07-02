// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// Payloads represents persistent storage for payloads.
type Payloads interface {

	// ByBlockID returns the payload with the given hash. It is available for
	// finalized and ambiguous blocks.
	ByBlockID(blockID flow.Identifier) (*flow.Payload, error)
}
