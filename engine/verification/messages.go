package verification

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// VerifiableChunk represents a ready-to-verify chunk
// It contains the execution result as well as all resources needed to verify it
type VerifiableChunk struct {
	ChunkIndex    uint64                 // index of the chunk to be verified
	EndState      flow.StateCommitment   // state commitment at the end of this chunk
	Block         *flow.Block            // block that contains this chunk
	Receipt       *flow.ExecutionReceipt // execution receipt of this block
	Collection    *flow.Collection       // collection corresponding to the chunk
	ChunkDataPack *flow.ChunkDataPack    // chunk data package needed to verify this chunk
}
