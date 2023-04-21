package unittest

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/utils/unittest"
)

func StateInteractionsFixture() *state.ExecutionSnapshot {
	return &state.ExecutionSnapshot{}
}

func ComputationResultFixture(
	parentBlockExecutionResultID flow.Identifier,
	collectionsSignerIDs [][]flow.Identifier,
) *execution.ComputationResult {

	startState := unittest.StateCommitmentFixture()
	block := unittest.ExecutableBlockFixture(collectionsSignerIDs, &startState)

	return ComputationResultForBlockFixture(
		parentBlockExecutionResultID,
		block)
}

func ComputationResultForBlockFixture(
	parentBlockExecutionResultID flow.Identifier,
	completeBlock *entity.ExecutableBlock,
) *execution.ComputationResult {
	collections := completeBlock.Collections()
	computationResult := execution.NewEmptyComputationResult(completeBlock)

	numberOfChunks := len(collections) + 1
	for i := 0; i < numberOfChunks; i++ {
		computationResult.CollectionExecutionResultAt(i).UpdateExecutionSnapshot(StateInteractionsFixture())
		computationResult.AppendCollectionAttestationResult(
			*completeBlock.StartState,
			*completeBlock.StartState,
			nil,
			unittest.IdentifierFixture(),
			nil,
		)

	}

	executionResult := flow.NewExecutionResult(
		parentBlockExecutionResultID,
		completeBlock.ID(),
		computationResult.AllChunks(),
		nil,
		flow.ZeroID)

	computationResult.ExecutionReceipt = &flow.ExecutionReceipt{
		ExecutionResult:   *executionResult,
		Spocks:            make([]crypto.Signature, numberOfChunks),
		ExecutorSignature: crypto.Signature{},
	}

	return computationResult
}
