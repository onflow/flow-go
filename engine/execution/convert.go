package execution

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

func GenerateExecutionResultAndChunkDataPacks(
	metrics module.ExecutionMetrics,
	prevResultId flow.Identifier,
	startState flow.StateCommitment,
	result *ComputationResult,
) (
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

	executionResult = flow.NewExecutionResult(
		prevResultId,
		blockID,
		chunks,
		result.ConvertedServiceEvents,
		result.ExecutionDataID)

	return endState, chdps, executionResult, nil
}
