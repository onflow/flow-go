package unittest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/utils/unittest"
)

func StateInteractionsFixture() *snapshot.ExecutionSnapshot {
	return &snapshot.ExecutionSnapshot{
		Meter: meter.NewMeter(meter.DefaultParameters()),
	}
}

func ComputationResultFixture(
	t *testing.T,
	parentBlockExecutionResultID flow.Identifier,
	collectionsSignerIDs [][]flow.Identifier,
) *execution.ComputationResult {

	startState := unittest.StateCommitmentFixture()
	block := unittest.ExecutableBlockFixture(collectionsSignerIDs, &startState)

	return ComputationResultForBlockFixture(t,
		parentBlockExecutionResultID,
		block)
}

func ComputationResultForBlockFixture(
	t *testing.T,
	parentBlockExecutionResultID flow.Identifier,
	completeBlock *entity.ExecutableBlock,
) *execution.ComputationResult {
	collections := completeBlock.Collections()
	computationResult := execution.NewEmptyComputationResult(completeBlock)

	numberOfChunks := len(collections) + 1
	ceds := make([]*execution_data.ChunkExecutionData, numberOfChunks)
	startState := *completeBlock.StartState
	for i := 0; i < numberOfChunks; i++ {
		ceds[i] = unittest.ChunkExecutionDataFixture(t, 1024)
		endState := unittest.StateCommitmentFixture()
		computationResult.CollectionExecutionResultAt(i).UpdateExecutionSnapshot(StateInteractionsFixture())
		computationResult.AppendCollectionAttestationResult(
			startState,
			endState,
			nil,
			unittest.IdentifierFixture(),
			ceds[i],
		)
		startState = endState
	}
	bed := unittest.BlockExecutionDataFixture(
		unittest.WithBlockExecutionDataBlockID(completeBlock.Block.ID()),
		unittest.WithChunkExecutionDatas(ceds...),
	)
	executionDataID, err := execution_data.CalculateID(context.Background(), bed, execution_data.DefaultSerializer)
	require.NoError(t, err)

	_, serviceEventEpochCommitProtocol := unittest.EpochCommitFixtureByChainID(flow.Localnet)
	_, serviceEventEpochSetupProtocol := unittest.EpochSetupFixtureByChainID(flow.Localnet)
	_, serviceEventEpochRecoverProtocol := unittest.EpochRecoverFixtureByChainID(flow.Localnet)
	_, serviceEventVersionBeaconProtocol := unittest.VersionBeaconFixtureByChainID(flow.Localnet)

	convertedServiceEvents := flow.ServiceEventList{
		serviceEventEpochCommitProtocol.ServiceEvent(),
		serviceEventEpochSetupProtocol.ServiceEvent(),
		serviceEventEpochRecoverProtocol.ServiceEvent(),
		serviceEventVersionBeaconProtocol.ServiceEvent(),
	}

	chunks, err := computationResult.AllChunks()
	require.NoError(t, err)

	executionResult, err := flow.NewExecutionResult(flow.UntrustedExecutionResult{
		PreviousResultID: parentBlockExecutionResultID,
		BlockID:          completeBlock.BlockID(),
		Chunks:           chunks,
		ServiceEvents:    convertedServiceEvents,
		ExecutionDataID:  executionDataID,
	})
	require.NoError(t, err)

	unsignedExecutionReceipt, err := flow.NewUnsignedExecutionReceipt(
		flow.UntrustedUnsignedExecutionReceipt{
			ExecutionResult: *executionResult,
			ExecutorID:      unittest.IdentifierFixture(),
			Spocks:          unittest.SignaturesFixture(numberOfChunks),
		},
	)
	require.NoError(t, err)
	receipt, err := flow.NewExecutionReceipt(
		flow.UntrustedExecutionReceipt{
			UnsignedExecutionReceipt: *unsignedExecutionReceipt,
			ExecutorSignature:        unittest.SignatureFixture(),
		},
	)
	require.NoError(t, err)
	computationResult.ExecutionReceipt = receipt

	return computationResult
}
