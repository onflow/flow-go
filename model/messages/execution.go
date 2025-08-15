package messages

import (
	"github.com/onflow/flow-go/model/flow"
)

// ChunkDataRequest represents a request for the a chunk data pack
// which is specified by a chunk ID.
type ChunkDataRequest struct {
	ChunkID flow.Identifier
	Nonce   uint64 // so that we aren't deduplicated by the network layer
}

var _ UntrustedMessage = (*ChunkDataRequest)(nil)

func (dr *ChunkDataRequest) ToInternal() (any, error) {
	// Temporary: just return the unvalidated wire struct
	return dr, nil
}

// ChunkDataResponse is the response to a chunk data pack request.
// It contains the chunk data pack of the interest.
type ChunkDataResponse struct {
	ChunkDataPack flow.ChunkDataPack
	Nonce         uint64 // so that we aren't deduplicated by the network layer
}

var _ UntrustedMessage = (*ChunkDataResponse)(nil)

func (dr *ChunkDataResponse) ToInternal() (any, error) {
	// Temporary: just return the unvalidated wire struct
	return dr, nil
}

type ExecutionReceipt flow.UntrustedExecutionReceipt

var _ UntrustedMessage = (*ExecutionReceipt)(nil)

func (er *ExecutionReceipt) ToInternal() (any, error) {
	// Temporary: just return the unvalidated wire struct
	return er, nil
}

type ResultApproval flow.UntrustedResultApproval

var _ UntrustedMessage = (*ResultApproval)(nil)

func (ra *ResultApproval) ToInternal() (any, error) {

	// Temporary: just return the unvalidated wire struct
	return ra, nil
}
