package access

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"

	accessmock "github.com/onflow/flow-go/engine/access/mock"
	"github.com/onflow/flow-go/engine/access/rpc"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storagemock "github.com/onflow/flow-go/storage/mock"
	grpcutils "github.com/onflow/flow-go/utils/grpc"
	"github.com/onflow/flow-go/utils/unittest"
)

type RateLimitTestSuite struct {
	suite.Suite
	state      *protocol.State
	snapshot   *protocol.Snapshot
	epochQuery *protocol.EpochQuery
	log        zerolog.Logger
	net        *module.Network
	request    *module.Requester
	collClient *accessmock.AccessAPIClient
	execClient *accessmock.ExecutionAPIClient
	me         *module.Local
	chainID    flow.ChainID
	metrics    *metrics.NoopCollector
	backend    *backend.Backend
	rpcEng     *rpc.Engine
	client     accessproto.AccessAPIClient
	closer     io.Closer

	// storage
	blocks       *storagemock.Blocks
	headers      *storagemock.Headers
	collections  *storagemock.Collections
	transactions *storagemock.Transactions
	receipts     *storagemock.ExecutionReceipts
}

func (suite *RateLimitTestSuite) SetupTest() {
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
	suite.execClient = new(accessmock.ExecutionAPIClient)

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
		GRPCListenAddr: "127.0.0.1:0",
	}
	suite.rpcEng = rpc.New(suite.log, suite.state, config, nil, nil, nil, suite.blocks, suite.headers, suite.collections, suite.transactions,
		nil, suite.chainID, suite.metrics, 0, 0, false, false)
	<-suite.rpcEng.Ready()

	// wait for the server to startup
	assert.Eventually(suite.T(), func() bool {
		return suite.rpcEng.GRPCAddress() != nil
	}, 5*time.Second, 10*time.Millisecond)

	// create the access api client
	var err error
	suite.client, suite.closer, err = accessAPIClient(suite.rpcEng.GRPCAddress().String())
	assert.NoError(suite.T(), err)
}

func (suite *RateLimitTestSuite) TearDownTest() {
	if suite.closer != nil {
		suite.closer.Close()
	}
}

func TestRateLimit(t *testing.T) {
	suite.Run(t, new(RateLimitTestSuite))
}

func (suite *RateLimitTestSuite) TestBasicRatelimiting() {
	assert.True(suite.T(), true)
	req := &accessproto.PingRequest{}
	ctx := context.Background()
	resp, err := suite.client.Ping(ctx, req)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)

}

func accessAPIClient(address string) (accessproto.AccessAPIClient, io.Closer, error) {
	conn, err := grpc.Dial(
		address,
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(grpcutils.DefaultMaxMsgSize)),
		grpc.WithInsecure())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to address %s: %w", address, err)
	}

	client := accessproto.NewAccessAPIClient(conn)
	closer := io.Closer(conn)
	return client, closer, nil
}
