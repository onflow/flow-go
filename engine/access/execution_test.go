package access

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"

	accessmock "github.com/onflow/flow-go/engine/access/mock"
	"github.com/onflow/flow-go/engine/access/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// ExecutionTestSuite tests that the Access node provides an execution API server
type ExecutionAPITestSuite struct {
	suite.Suite
	state      *protocol.State
	snapshot   *protocol.Snapshot
	epochQuery *protocol.EpochQuery
	log        zerolog.Logger
	net        *module.Network
	request    *module.Requester
	me         *module.Local
	chainID    flow.ChainID
	metrics    *metrics.NoopCollector
	rpcEng     *rpc.Engine
	collClient *accessmock.AccessAPIClient

	// storage
	blocks       *storagemock.Blocks
	headers      *storagemock.Headers
	collections  *storagemock.Collections
	transactions *storagemock.Transactions
	receipts     *storagemock.ExecutionReceipts
}

func (suite *ExecutionAPITestSuite) SetupTest() {
	suite.log = zerolog.New(os.Stdout)
	suite.net = new(module.Network)
	suite.state = new(protocol.State)
	suite.snapshot = new(protocol.Snapshot)

	suite.epochQuery = new(protocol.EpochQuery)
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Epochs").Return(suite.epochQuery).Maybe()
	suite.blocks = new(storagemock.Blocks)
	suite.headers = new(storagemock.Headers)
	suite.transactions = new(storagemock.Transactions)
	suite.collections = new(storagemock.Collections)
	suite.receipts = new(storagemock.ExecutionReceipts)

	suite.collClient = new(accessmock.AccessAPIClient)

	suite.request = new(module.Requester)
	suite.request.On("EntityByID", mock.Anything, mock.Anything)

	suite.me = new(module.Local)

	accessIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleAccess))
	suite.me.
		On("NodeID").
		Return(accessIdentity.NodeID)

	suite.chainID = flow.Testnet
	suite.metrics = metrics.NewNoopCollector()

	config := rpc.Config{
		UnsecureGRPCListenAddr: ":0", // :0 to let the OS pick a free port
		SecureGRPCListenAddr:   ":0",
		HTTPListenAddr:         ":0",
	}

	suite.rpcEng = rpc.New(suite.log, suite.state, config, suite.collClient, nil, suite.blocks, suite.headers, suite.collections, suite.transactions,
		nil, suite.chainID, suite.metrics, 0, 0, false, false, nil, nil, true)
	unittest.AssertClosesBefore(suite.T(), suite.rpcEng.Ready(), 2*time.Second)

	// wait for the server to startup
	assert.Eventually(suite.T(), func() bool {
		return suite.rpcEng.UnsecureGRPCAddress() != nil
	}, 5*time.Second, 10*time.Millisecond)
}

func TestExecutionAPI(t *testing.T) {
	suite.Run(t, new(ExecutionAPITestSuite))
}

func (suite *ExecutionAPITestSuite) TestExecutionAPICall() {

	req := &execution.PingRequest{}
	ctx := context.Background()

	suite.collClient.On("Ping", mock.Anything, mock.Anything).Return(nil, nil).Once()

	suite.Run("happy path - grpc client can connect successfully", func() {
		client, closer := suite.executionAPIClient()
		defer closer.Close()
		resp, err := client.Ping(ctx, req)
		assert.NoError(suite.T(), err)
		assert.Equal(suite.T(), &execution.PingResponse{}, resp)
		suite.collClient.AssertExpectations(suite.T())
	})
}

func (suite *ExecutionAPITestSuite) TearDownTest() {
	// close the server
	if suite.rpcEng != nil {
		unittest.AssertClosesBefore(suite.T(), suite.rpcEng.Done(), 2*time.Second)
	}
}

// executionAPIClient creates an execution API client using the given public key
func (suite *ExecutionAPITestSuite) executionAPIClient() (execution.ExecutionAPIClient, io.Closer) {
	conn, err := grpc.Dial(suite.rpcEng.UnsecureGRPCAddress().String(), grpc.WithInsecure())
	assert.NoError(suite.T(), err)

	client := execution.NewExecutionAPIClient(conn)
	closer := io.Closer(conn)
	return client, closer
}
