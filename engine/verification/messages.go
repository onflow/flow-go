package verification

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// VerifiableChunkData represents a ready-to-verify chunk
// It contains the execution result as well as all resources needed to verify it
type VerifiableChunkData struct {
	Chunk         *flow.Chunk           // the chunk to be verified
	Header        *flow.Header          // BlockHeader that contains this chunk
	Result        *flow.ExecutionResult // execution result of this block
	Collection    *flow.Collection      // collection corresponding to the chunk
	ChunkDataPack *flow.ChunkDataPack   // chunk data package needed to verify this chunk
	EndState      flow.StateCommitment  // state commitment at the end of this chunk
}
