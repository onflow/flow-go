package execution

import (
	"bytes"
	"context"
	"testing"

	"github.com/onflow/flow/protobuf/go/flow/execution"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-go/integration/testnet"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	sdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go/integration/tests/lib"
)

func TestTransactionMetrics(t *testing.T) {
	suite.Run(t, new(TransactionMetricsSuite))
}

type TransactionMetricsSuite struct {
	Suite
}

func (s *TransactionMetricsSuite) TestTransactionMetrics() {
	accessClient := s.AccessClient()

	// wait for next height finalized (potentially first height), called blockA
	currentFinalized := s.BlockState.HighestFinalizedHeight()
	blockA := s.BlockState.WaitForHighestFinalizedProgress(s.T(), currentFinalized)
	s.T().Logf("got blockA height %v ID %v\n", blockA.Header.Height, blockA.Header.ID())

	// send transaction
	tx, err := accessClient.DeployContract(context.Background(), sdk.Identifier(s.net.Root().ID()), lib.CounterContract)
	require.NoError(s.T(), err, "could not deploy counter")

	txres, err := accessClient.WaitForExecuted(context.Background(), tx.ID())
	require.NoError(s.T(), err, "could not wait for tx to be executed")
	require.NoError(s.T(), txres.Error)

	client, closeClient := s.getClient()
	defer func() {
		_ = closeClient()
	}()

	result, err := client.GetTransactionExecutionMetricsAfter(
		context.Background(),
		&execution.GetTransactionExecutionMetricsAfterRequest{
			BlockHeight: 0,
		},
	)

	require.NoError(s.T(), err, "could not get transaction execution metrics")
	require.NotNil(s.T(), result.Results)
	// there should be at least some results, due to each block having at least 1 transaction
	require.Greater(s.T(), len(result.Results), 10)

	latestBlockResult := uint64(0)
	for _, result := range result.Results {
		if result.BlockHeight > latestBlockResult {
			latestBlockResult = result.BlockHeight
		}
	}

	// send another transaction
	tx, err = accessClient.UpdateContract(context.Background(), sdk.Identifier(s.net.Root().ID()), lib.CounterContract)
	require.NoError(s.T(), err, "could not deploy counter")

	txres, err = accessClient.WaitForExecuted(context.Background(), tx.ID())
	require.NoError(s.T(), err, "could not wait for tx to be executed")
	require.NoError(s.T(), txres.Error)

	result, err = client.GetTransactionExecutionMetricsAfter(
		context.Background(),
		&execution.GetTransactionExecutionMetricsAfterRequest{
			BlockHeight: latestBlockResult,
		},
	)

	require.NoError(s.T(), err, "could not get transaction execution metrics")
	// there could be only 1 block since the last time
	require.Greater(s.T(), len(result.Results), 0)

	transactionExists := false
	for _, result := range result.Results {
		for _, transaction := range result.Transactions {
			if bytes.Equal(transaction.TransactionId, tx.ID().Bytes()) {
				transactionExists = true

				// check that the transaction metrics are not 0
				require.Greater(s.T(), transaction.ExecutionTime, uint64(0))
				require.Greater(s.T(), len(transaction.ExecutionEffortWeights), 0)
			}
		}
		require.Less(s.T(), latestBlockResult, result.BlockHeight)

	}
	require.True(s.T(), transactionExists)
}

func (s *TransactionMetricsSuite) getClient() (execution.ExecutionAPIClient, func() error) {

	exe1ID := s.net.ContainerByID(s.exe1ID)
	addr := exe1ID.Addr(testnet.GRPCPort)

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(s.T(), err, "could not create execution client")

	grpcClient := execution.NewExecutionAPIClient(conn)
	return grpcClient, conn.Close
}
