package convert

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
	"github.com/onflow/flow-go/state/protocol"
)

func FromChunkDataPack(
	chunk *flow.Chunk,
	chunkDataPack *flow.ChunkDataPack,
	header *flow.Header,
	snapshot protocol.Snapshot,
	result *flow.ExecutionResult,
) (*verification.VerifiableChunkData, error) {

	// system chunk is the last chunk
	isSystemChunk := IsSystemChunk(chunk.Index, result)

	endState, err := EndStateCommitment(result, chunk.Index, isSystemChunk)
	if err != nil {
		return nil, fmt.Errorf("could not compute end state of chunk: %w", err)
	}

	transactionOffset, err := TransactionOffsetForChunk(result.Chunks, chunk.Index)
	if err != nil {
		return nil, fmt.Errorf("cannot compute transaction offset for chunk: %w", err)
	}

	return &verification.VerifiableChunkData{
		IsSystemChunk:     isSystemChunk,
		Chunk:             chunk,
		Header:            header,
		Snapshot:          snapshot,
		Result:            result,
		ChunkDataPack:     chunkDataPack,
		EndState:          endState,
		TransactionOffset: transactionOffset,
	}, nil
}

// EndStateCommitment computes the end state of the given chunk.
func EndStateCommitment(result *flow.ExecutionResult, chunkIndex uint64, systemChunk bool) (flow.StateCommitment, error) {
	var endState flow.StateCommitment
	if systemChunk {
		var err error
		// last chunk in a result is the system chunk and takes final state commitment
		endState, err = result.FinalStateCommitment()
		if err != nil {
			return flow.DummyStateCommitment, fmt.Errorf("can not read final state commitment, likely a bug:%w", err)
		}
	} else {
		// any chunk except last takes the subsequent chunk's start state
		endState = result.Chunks[chunkIndex+1].StartState
	}

	return endState, nil
}

// TransactionOffsetForChunk calculates transaction offset for a given chunk which is the index of the first
// transaction of this chunk within the whole block
func TransactionOffsetForChunk(chunks flow.ChunkList, chunkIndex uint64) (uint32, error) {
	if int(chunkIndex) > len(chunks)-1 {
		return 0, fmt.Errorf("chunk list out of bounds, len %d asked for chunk %d", len(chunks), chunkIndex)
	}
	var offset uint32 = 0
	for i := 0; i < int(chunkIndex); i++ {
		offset += uint32(chunks[i].NumberOfTransactions)
	}
	return offset, nil
}

// IsSystemChunk returns true if `chunkIndex` points to a system chunk in `result`.
// Otherwise, it returns false.
// In the current version, a chunk is a system chunk if it is the last chunk of the
// execution result.
func IsSystemChunk(chunkIndex uint64, result *flow.ExecutionResult) bool {
	return chunkIndex == uint64(len(result.Chunks)-1)
}
