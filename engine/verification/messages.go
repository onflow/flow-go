package verification

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// CompleteExecutionResult represents an execution result that is ready to
// be verified. It contains all execution result and all resources required to
// verify it.
// TODO update this as needed based on execution requirements
type CompleteExecutionResult struct {
	Receipt        *flow.ExecutionReceipt
	Block          *flow.Block
	Collections    []*flow.Collection
	ChunkStates    []*flow.ChunkState
	ChunkDataPacks []*flow.ChunkDataPack
}

// VerifiableChunk represents a ready-to-verify chunk
// It contains the execution result as well as all resources needed to verify it
type VerifiableChunk struct {
	ChunkIndex    uint64                 // index of the chunk to be verified
	EndState      flow.StateCommitment   // state commitment at the end of this chunk
	Block         *flow.Block            // block that contains this chunk
	Receipt       *flow.ExecutionReceipt // execution receipt of this block
	Collection    *flow.Collection       // collection corresponding to the chunk
	ChunkState    *flow.ChunkState       // state of registers corresponding to the chunk
	ChunkDataPack *flow.ChunkDataPack
}
