package execution

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/model/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/state_synchronization"
)

func GenerateExecutionResultAndChunkDataPacks(
	ctx context.Context,
	eds state_synchronization.ExecutionDataService,
	edCache state_synchronization.ExecutionDataCIDCache,
	prevResultId flow.Identifier,
	startState flow.StateCommitment,
	result *ComputationResult) (
	endState flow.StateCommitment,
	chdps []*flow.ChunkDataPack,
	executionResult *flow.ExecutionResult,
	err error,
) {

	// no need to persist the state interactions, since they are used only by state
	// syncing, which is currently disabled
	block := result.ExecutableBlock.Block
	blockID := block.ID()

	chunks := make([]*flow.Chunk, len(result.StateCommitments))
	chdps = make([]*flow.ChunkDataPack, len(result.StateCommitments))

	// TODO: check current state root == startState
	endState = startState

	for i := range result.StateCommitments {
		// TODO: deltas should be applied to a particular state

		endState = result.StateCommitments[i]
		var chunk *flow.Chunk

		var collection *flow.Collection
		numTransactions := 1

		// account for system chunk being last
		if i < len(result.StateCommitments)-1 {
			// non-system chunks
			collectionGuarantee := result.ExecutableBlock.Block.Payload.Guarantees[i]
			completeCollection := result.ExecutableBlock.CompleteCollections[collectionGuarantee.ID()]
			c := completeCollection.Collection()
			collection = &c
			numTransactions = len(completeCollection.Transactions)
		}

		ed := &state_synchronization.ExecutionData{
			Collection: collection,
			Events:     result.Events[i],
			TrieUpdate: result.TrieUpdates[i],
		}

		executionDataID, blobTree, err := eds.Add(ctx, ed)
		if err != nil {
			return flow.DummyStateCommitment, nil, nil, fmt.Errorf("could not add Execution Data: %w", err)
		}

		edCache.Insert(blockID, block.Header.Height, i, blobTree)

		chunk = GenerateChunk(i, startState, endState, blockID, result.EventsHashes[i], uint64(numTransactions), executionDataID)
		chdps[i] = GenerateChunkDataPack(chunk.ID(), startState, collection, result.Proofs[i])

		// TODO use view.SpockSecret() as an input to spock generator
		chunks[i] = chunk
		startState = endState
	}

	executionResult, err = GenerateExecutionResultForBlock(prevResultId, block, chunks, result.ServiceEvents, result.ExecutionDataID)
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
	executionDataID flow.Identifier,
) (*flow.ExecutionResult, error) {

	// convert Cadence service event representation to flow-go representation
	convertedServiceEvents := make([]flow.ServiceEvent, 0, len(serviceEvents))
	for _, event := range serviceEvents {
		converted, err := convert.ServiceEvent(block.Header.ChainID, event)
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
	blockID, eventsCollection flow.Identifier,
	txNumber uint64, executionDataID flow.Identifier,
) *flow.Chunk {
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
		Index:           uint64(colIndex),
		EndState:        endState,
		ExecutionDataID: executionDataID,
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
