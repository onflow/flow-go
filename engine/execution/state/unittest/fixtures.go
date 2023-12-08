package unittest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/crypto"
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
	for i := 0; i < numberOfChunks; i++ {
		ceds[i] = unittest.ChunkExecutionDataFixture(t, 1024)
		computationResult.CollectionExecutionResultAt(i).UpdateExecutionSnapshot(StateInteractionsFixture())
		computationResult.AppendCollectionAttestationResult(
			*completeBlock.StartState,
			*completeBlock.StartState,
			nil,
			unittest.IdentifierFixture(),
			ceds[i],
		)
	}
	bed := unittest.BlockExecutionDataFixture(
		unittest.WithBlockExecutionDataBlockID(completeBlock.Block.ID()),
		unittest.WithChunkExecutionDatas(ceds...),
	)
	executionDataID, err := execution_data.CalculateID(context.Background(), bed, execution_data.DefaultSerializer)
	require.NoError(t, err)

	_, serviceEventEpochCommitProtocol := unittest.EpochCommitFixtureByChainID(flow.Localnet)
	_, serviceEventEpochSetupProtocol := unittest.EpochSetupFixtureByChainID(flow.Localnet)
	_, serviceEventVersionBeaconProtocol := unittest.VersionBeaconFixtureByChainID(flow.Localnet)

	convertedServiceEvents := flow.ServiceEventList{
		serviceEventEpochCommitProtocol.ServiceEvent(),
		serviceEventEpochSetupProtocol.ServiceEvent(),
		serviceEventVersionBeaconProtocol.ServiceEvent(),
	}

	executionResult := flow.NewExecutionResult(
		parentBlockExecutionResultID,
		completeBlock.ID(),
		computationResult.AllChunks(),
		convertedServiceEvents,
		executionDataID)

	computationResult.ExecutionReceipt = &flow.ExecutionReceipt{
		ExecutionResult:   *executionResult,
		Spocks:            make([]crypto.Signature, numberOfChunks),
		ExecutorSignature: crypto.Signature{},
	}

	return computationResult
}
