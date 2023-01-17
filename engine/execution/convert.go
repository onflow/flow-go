package execution

import (
	"fmt"

	"github.com/onflow/flow-go/model/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

func GenerateExecutionResultAndChunkDataPacks(
	metrics module.ExecutionMetrics,
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
		// account for system chunk being last
		if i < len(result.StateCommitments)-1 {
			// non-system chunks
			collectionGuarantee := result.ExecutableBlock.Block.Payload.Guarantees[i]
			completeCollection := result.ExecutableBlock.CompleteCollections[collectionGuarantee.ID()]
			collection := completeCollection.Collection()
			chunk = flow.NewChunk(
				blockID,
				i,
				startState,
				len(completeCollection.Transactions),
				result.EventsHashes[i],
				endState)
			chdps[i] = flow.NewChunkDataPack(
				chunk.ID(),
				startState,
				result.Proofs[i],
				&collection)
			metrics.ExecutionChunkDataPackGenerated(len(result.Proofs[i]), len(completeCollection.Transactions))

		} else {
			// system chunk
			// note that system chunk does not have a collection.
			// also, number of transactions is one for system chunk.
			chunk = flow.NewChunk(
				blockID,
				i,
				startState,
				1,
				result.EventsHashes[i],
				endState)
			// system chunk has a nil collection.
			chdps[i] = flow.NewChunkDataPack(
				chunk.ID(),
				startState,
				result.Proofs[i],
				nil)
			metrics.ExecutionChunkDataPackGenerated(len(result.Proofs[i]), 1)
		}

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
		ExecutionDataID:  executionDataID,
	}

	return er, nil
}
