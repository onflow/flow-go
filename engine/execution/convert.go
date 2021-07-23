package execution

import (
	"fmt"

	"github.com/onflow/flow-go/model/convert"
	"github.com/onflow/flow-go/model/flow"
	storagemodel "github.com/onflow/flow-go/storage/model"
)

func GenerateExecutionResultAndChunkDataPacks(
	prevResultId flow.Identifier,
	startState flow.StateCommitment,
	result *ComputationResult) (
	endState flow.StateCommitment,
	chdps []*storagemodel.StoredChunkDataPack,
	executionResult *flow.ExecutionResult,
	err error,
) {

	// no need to persist the state interactions, since they are used only by state
	// syncing, which is currently disabled
	block := result.ExecutableBlock.Block
	blockID := block.ID()

	chunks := make([]*flow.Chunk, len(result.StateCommitments))
	chdps = make([]*storagemodel.StoredChunkDataPack, len(result.StateCommitments))

	// TODO: check current state root == startState
	endState = startState

	for i := range result.StateCommitments {
		// TODO: deltas should be applied to a particular state

		endState = result.StateCommitments[i]
		var chunk *flow.Chunk
		// account for system chunk being last
		if i < len(result.StateCommitments)-1 {
			collectionGuarantee := result.ExecutableBlock.Block.Payload.Guarantees[i]
			completeCollection := result.ExecutableBlock.CompleteCollections[collectionGuarantee.ID()]
			collectionID := completeCollection.Collection().ID()
			chunk = GenerateChunk(i, startState, endState, blockID, result.EventsHashes[i], uint16(len(completeCollection.Transactions)))
			chdps[i] = GenerateStoredChunkDataPack(chunk, &collectionID, result.Proofs[i])
		} else {
			chunk = GenerateChunk(i, startState, endState, blockID, result.EventsHashes[i], 1) // for system chunk
			chdps[i] = GenerateStoredChunkDataPack(chunk, nil, result.Proofs[i])
		}

		// eventsHash := result.EventsHashes[i]
		// chunk := GenerateChunk(i, startState, endState, blockID, eventsHash, uint16(txNumber))

		// chunkDataPack
		// chdps[i] = GenerateStoredChunkDataPack(chunk, &collectionID, result.Proofs[i])
		// TODO use view.SpockSecret() as an input to spock generator
		chunks[i] = chunk
		startState = endState
	}

	executionResult, err = GenerateExecutionResultForBlock(prevResultId, block, chunks, result.ServiceEvents)
	if err != nil {
		return flow.DummyStateCommitment, nil, nil, fmt.Errorf("could not generate execution result: %w", err)
	}

	return endState, chdps, executionResult, nil
}

// GenerateExecutionResultForBlock creates new ExecutionResult for a block from
// the provided chunk results.
func GenerateExecutionResultForBlock(
	previousErID flow.Identifier,
	block *flow.Block,
	chunks []*flow.Chunk,
	serviceEvents []flow.Event,
) (*flow.ExecutionResult, error) {

	// convert Cadence service event representation to flow-go representation
	convertedServiceEvents := make([]flow.ServiceEvent, 0, len(serviceEvents))
	for _, event := range serviceEvents {
		converted, err := convert.ServiceEvent(event)
		if err != nil {
			return nil, fmt.Errorf("could not convert service event: %w", err)
		}
		convertedServiceEvents = append(convertedServiceEvents, *converted)
	}

	er := &flow.ExecutionResult{
		PreviousResultID: previousErID,
		BlockID:          block.ID(),
		Chunks:           chunks,
		ServiceEvents:    convertedServiceEvents,
	}

	return er, nil
}

// GenerateChunk creates a chunk from the provided computation data.
func GenerateChunk(colIndex int,
	startState, endState flow.StateCommitment,
	blockID, eventsCollection flow.Identifier, txNumber uint16) *flow.Chunk {
	return &flow.Chunk{
		ChunkBody: flow.ChunkBody{
			CollectionIndex: uint(colIndex),
			StartState:      startState,
			EventCollection: eventsCollection,
			BlockID:         blockID,
			// TODO: record gas used
			TotalComputationUsed: 0,
			NumberOfTransactions: txNumber,
		},
		Index:    uint64(colIndex),
		EndState: endState,
	}
}

// GenerateStoredChunkDataPack generates a stored chunk data pack for persisting in storage.
func GenerateStoredChunkDataPack(
	chunk *flow.Chunk,
	collectionID *flow.Identifier,
	proof flow.StorageProof,
) *storagemodel.StoredChunkDataPack {
	return &storagemodel.StoredChunkDataPack{
		ChunkID:      chunk.ID(),
		StartState:   chunk.StartState,
		Proof:        proof,
		CollectionID: collectionID,
	}
}

func GenerateChunkDataPack(
	chunkID flow.Identifier,
	startState flow.StateCommitment,
	collection *flow.Collection,
	proof flow.StorageProof,
) *flow.ChunkDataPack {
	return &flow.ChunkDataPack{
		ChunkID:    chunkID,
		StartState: startState,
		Proof:      proof,
		Collection: collection,
	}
}

func GenerateSystemChunkDataPack(
	chunkID flow.Identifier,
	startState flow.StateCommitment,
	proof flow.StorageProof,
) *flow.ChunkDataPack {
	return &flow.ChunkDataPack{
		ChunkID:    chunkID,
		StartState: startState,
		Proof:      proof,
	}
}

// ToChunkDataPack converts a chunk data pack from its storage model to the form that can be sent over wire.
func ToChunkDataPack(storedChunkDataPack *storagemodel.StoredChunkDataPack, collection *flow.Collection) *flow.ChunkDataPack {
	if collection != nil {
		// non-system chunk
		return GenerateChunkDataPack(storedChunkDataPack.ChunkID, storedChunkDataPack.StartState, collection, storedChunkDataPack.Proof)
	}

	// system chunk
	return GenerateSystemChunkDataPack(storedChunkDataPack.ChunkID, storedChunkDataPack.StartState, storedChunkDataPack.Proof)
}
