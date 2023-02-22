package unittest

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/utils/unittest"
)

func StateInteractionsFixture() *delta.SpockSnapshot {
	return delta.NewDeltaView(nil).Interactions()
}

func ComputationResultFixture(
	parentBlockExecutionResultID flow.Identifier,
	collectionsSignerIDs [][]flow.Identifier,
) *execution.ComputationResult {
	block := unittest.ExecutableBlockFixture(collectionsSignerIDs)
	startState := unittest.StateCommitmentFixture()
	block.StartState = &startState

	return ComputationResultForBlockFixture(
		parentBlockExecutionResultID,
		block)
}

func ComputationResultForBlockFixture(
	parentBlockExecutionResultID flow.Identifier,
	completeBlock *entity.ExecutableBlock,
) *execution.ComputationResult {
	collections := completeBlock.Collections()

	numChunks := len(collections) + 1
	stateViews := make([]*delta.SpockSnapshot, numChunks)
	stateCommitments := make([]flow.StateCommitment, numChunks)
	proofs := make([][]byte, numChunks)
	events := make([]flow.EventsList, numChunks)
	eventHashes := make([]flow.Identifier, numChunks)
	spockHashes := make([]crypto.Signature, numChunks)
	chunks := make([]*flow.Chunk, 0, numChunks)
	chunkDataPacks := make([]*flow.ChunkDataPack, 0, numChunks)
	chunkExecutionDatas := make(
		[]*execution_data.ChunkExecutionData,
		0,
		numChunks)
	for i := 0; i < numChunks; i++ {
		stateViews[i] = StateInteractionsFixture()
		stateCommitments[i] = *completeBlock.StartState
		proofs[i] = unittest.RandomBytes(6)
		events[i] = make(flow.EventsList, 0)
		eventHashes[i] = unittest.IdentifierFixture()

		chunk := flow.NewChunk(
			completeBlock.ID(),
			i,
			*completeBlock.StartState,
			0,
			unittest.IdentifierFixture(),
			*completeBlock.StartState)
		chunks = append(chunks, chunk)

		var collection *flow.Collection
		if i < len(collections) {
			colStruct := collections[i].Collection()
			collection = &colStruct
		}

		chunkDataPacks = append(
			chunkDataPacks,
			flow.NewChunkDataPack(
				chunk.ID(),
				*completeBlock.StartState,
				proofs[i],
				collection))

		chunkExecutionDatas = append(
			chunkExecutionDatas,
			&execution_data.ChunkExecutionData{
				Collection: collection,
				Events:     nil,
				TrieUpdate: nil,
			})
	}
	return &execution.ComputationResult{
		TransactionResultIndex: make([]int, numChunks),
		ExecutableBlock:        completeBlock,
		StateSnapshots:         stateViews,
		StateCommitments:       stateCommitments,
		Proofs:                 proofs,
		Events:                 events,
		EventsHashes:           eventHashes,
		SpockSignatures:        spockHashes,
		Chunks:                 chunks,
		ChunkDataPacks:         chunkDataPacks,
		EndState:               *completeBlock.StartState,
		BlockExecutionData: &execution_data.BlockExecutionData{
			BlockID:             completeBlock.ID(),
			ChunkExecutionDatas: chunkExecutionDatas,
		},
		ExecutionResult: flow.NewExecutionResult(
			parentBlockExecutionResultID,
			completeBlock.ID(),
			chunks,
			nil,
			flow.ZeroID),
	}
}
