package execution

import (
	"context"
	"testing"

	"github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	sdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/model/flow"
)

func TestGetExecutionResult(t *testing.T) {
	suite.Run(t, new(GetExecutionResultSuite))
}

type GetExecutionResultSuite struct {
	Suite
}

// TestGetExecutionResult verifies that the execution node correctly serves execution results
// via both the GetExecutionResultForBlockID and GetExecutionResultByID RPC endpoints, and
// that both endpoints return consistent data for the same underlying result.
func (s *GetExecutionResultSuite) TestGetExecutionResult() {
	ctx := context.Background()
	accessClient := s.AccessClient()

	// Deploy a contract to ensure a non-trivial block is executed.
	tx, err := accessClient.DeployContract(ctx, sdk.Identifier(s.net.Root().ID()), lib.CounterContract)
	require.NoError(s.T(), err, "could not deploy counter contract")

	txRes, err := accessClient.WaitForExecuted(ctx, tx.ID())
	require.NoError(s.T(), err, "could not wait for tx to be executed")
	require.NoError(s.T(), txRes.Error)

	blockID := flow.Identifier(txRes.BlockID)

	client, closeClient := s.getExecutionNodeClient()
	defer func() {
		s.Require().NoError(closeClient())
	}()

	// Fetch the execution result by block ID.
	byBlockResp, err := client.GetExecutionResultForBlockID(ctx, &execution.GetExecutionResultForBlockIDRequest{
		BlockId: blockID[:],
	})
	require.NoError(s.T(), err, "could not get execution result for block ID %s", blockID)

	// Convert the protobuf result to a flow model to compute the result ID.
	byBlockResult, err := convert.MessageToExecutionResult(byBlockResp.GetExecutionResult())
	require.NoError(s.T(), err, "could not convert execution result proto to flow model")

	byBlockResultID := byBlockResult.ID()

	// Fetch the same execution result by its ID.
	byIDResp, err := client.GetExecutionResultByID(ctx, &execution.GetExecutionResultByIDRequest{
		Id: byBlockResultID[:],
	})
	require.NoError(s.T(), err, "could not get execution result by ID %s", byBlockResultID)

	byIDResult, err := convert.MessageToExecutionResult(byIDResp.GetExecutionResult())
	require.NoError(s.T(), err, "could not convert execution result by ID proto to flow model")

	// Verify the constructed result IDs match
	require.Equal(s.T(), byBlockResult.BlockID, blockID, "result fetched by ID should have the requested block ID")
	require.Equal(s.T(), byBlockResultID, byIDResult.ID(), "result fetched by ID should have the same ID as result fetched by block ID")
}

func (s *GetExecutionResultSuite) getExecutionNodeClient() (execution.ExecutionAPIClient, func() error) {
	exe1Container := s.net.ContainerByID(s.exe1ID)
	addr := exe1Container.Addr(testnet.GRPCPort)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(s.T(), err, "could not create execution gRPC client")

	grpcClient := execution.NewExecutionAPIClient(conn)
	return grpcClient, conn.Close
}
