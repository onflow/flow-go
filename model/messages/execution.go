package messages

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// ChunkDataRequest represents a request for the chunk data pack
// which is specified by a chunk ID.
type ChunkDataRequest flow.ChunkDataRequest

// ToInternal returns the internal type representation for ChunkDataRequest.
//
// All errors indicate that the decode target contains a structurally invalid representation of the internal flow.ChunkDataRequest.
func (c *ChunkDataRequest) ToInternal() (any, error) {
	return (*flow.ChunkDataRequest)(c), nil
}

// ChunkDataResponse is the response to a chunk data pack request.
// It contains the chunk data pack of the interest.
type ChunkDataResponse struct {
	ChunkDataPack flow.UntrustedChunkDataPack
	Nonce         uint64 // so that we aren't deduplicated by the network layer
}

// ToInternal returns the internal type representation for ChunkDataResponse.
//
// All errors indicate that the decode target contains a structurally invalid representation of the internal flow.ChunkDataResponse.
func (c *ChunkDataResponse) ToInternal() (any, error) {
	chunkDataPack, err := flow.NewChunkDataPack(c.ChunkDataPack)
	if err != nil {
		return nil, fmt.Errorf("could not convert %T to internal type: %w", c.ChunkDataPack, err)
	}
	return &flow.ChunkDataResponse{
		Nonce:         c.Nonce,
		ChunkDataPack: *chunkDataPack,
	}, nil
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
