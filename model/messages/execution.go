package messages

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// ChunkDataRequest represents a request for the a chunk data pack
// which is specified by a chunk ID.
type ChunkDataRequest struct {
	ChunkID flow.Identifier
	Nonce   uint64 // so that we aren't deduplicated by the network layer
}

// ToInternal converts the untrusted ChunkDataRequest into its trusted internal
// representation.
//
// This stub returns the receiver unchanged. A proper implementation
// must perform validation checks and return a constructed internal
// object.
func (c *ChunkDataRequest) ToInternal() (any, error) {
	// TODO(malleability, #7715) implement with validation checks
	return c, nil
}

// ChunkDataResponse is the response to a chunk data pack request.
// It contains the chunk data pack of the interest.
type ChunkDataResponse struct {
	ChunkDataPack flow.ChunkDataPack
	Nonce         uint64 // so that we aren't deduplicated by the network layer
}

// ToInternal converts the untrusted ChunkDataResponse into its trusted internal
// representation.
//
// This stub returns the receiver unchanged. A proper implementation
// must perform validation checks and return a constructed internal
// object.
func (c *ChunkDataResponse) ToInternal() (any, error) {
	// TODO(malleability, #7716) implement with validation checks
	return c, nil
}

// ExecutionReceipt is the full execution receipt, as sent by the Execution Node.
// Specifically, it contains the detailed execution result.
type ExecutionReceipt flow.UntrustedExecutionReceipt

// ToInternal returns the internal type representation for ExecutionReceipt.
//
// All errors indicate that the decode target contains a structurally invalid representation of the internal flow.ExecutionReceipt.
func (er *ExecutionReceipt) ToInternal() (any, error) {
	internal, err := flow.NewExecutionReceipt(flow.UntrustedExecutionReceipt(*er))
	if err != nil {
		return nil, fmt.Errorf("could not convert %T to internal type: %w", er, err)
	}
	return internal, nil
}
