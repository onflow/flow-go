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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	accessmock "github.com/onflow/flow-go/engine/access/mock"
	"github.com/onflow/flow-go/engine/access/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/grpcutils"
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
	rpcEng     *rpc.Engine
	client     accessproto.AccessAPIClient
	closer     io.Closer

	// storage
	blocks       *storagemock.Blocks
	headers      *storagemock.Headers
	collections  *storagemock.Collections
	transactions *storagemock.Transactions
	receipts     *storagemock.ExecutionReceipts

	// test rate limit
	rateLimit  int
	burstLimit int
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
		UnsecureGRPCListenAddr: ":0", // :0 to let the OS pick a free port
		SecureGRPCListenAddr:   ":0",
		HTTPListenAddr:         ":0",
	}

	// set the rate limit to test with
	suite.rateLimit = 2
	// set the burst limit to test with
	suite.burstLimit = 2

	apiRateLimt := map[string]int{
		"Ping": suite.rateLimit,
	}

	apiBurstLimt := map[string]int{
		"Ping": suite.rateLimit,
	}

	suite.rpcEng = rpc.New(suite.log, suite.state, config, suite.collClient, nil, suite.blocks, suite.headers, suite.collections, suite.transactions,
		nil, nil, suite.chainID, suite.metrics, 0, 0, false, false, apiRateLimt, apiBurstLimt)
	unittest.AssertClosesBefore(suite.T(), suite.rpcEng.Ready(), 2*time.Second)

	// wait for the server to startup
	assert.Eventually(suite.T(), func() bool {
		return suite.rpcEng.UnsecureGRPCAddress() != nil
	}, 5*time.Second, 10*time.Millisecond)

	// create the access api client
	var err error
	suite.client, suite.closer, err = accessAPIClient(suite.rpcEng.UnsecureGRPCAddress().String())
	assert.NoError(suite.T(), err)
}

func (suite *RateLimitTestSuite) TearDownTest() {
	// close the client
	if suite.closer != nil {
		suite.closer.Close()
	}
	// close the server
	if suite.rpcEng != nil {
		unittest.AssertClosesBefore(suite.T(), suite.rpcEng.Done(), 2*time.Second)
	}
}

func TestRateLimit(t *testing.T) {
	suite.Run(t, new(RateLimitTestSuite))
}

// TestRatelimitingWithoutBurst tests that rate limit is correctly applied to an Access API call
func (suite *RateLimitTestSuite) TestRatelimitingWithoutBurst() {

	req := &accessproto.PingRequest{}
	ctx := context.Background()

	// expect 2 upstream calls
	suite.execClient.On("Ping", mock.Anything, mock.Anything).Return(nil, nil).Times(suite.rateLimit)
	suite.collClient.On("Ping", mock.Anything, mock.Anything).Return(nil, nil).Times(suite.rateLimit)

	requestCnt := 0
	// requests within the burst should succeed
	for requestCnt < suite.rateLimit {
		resp, err := suite.client.Ping(ctx, req)
		assert.NoError(suite.T(), err)
		assert.NotNil(suite.T(), resp)
		// sleep to prevent burst
		time.Sleep(100 * time.Millisecond)
		requestCnt++
	}

	// request more than the limit should fail
	_, err := suite.client.Ping(ctx, req)
	suite.assertRateLimitError(err)
}

// TestRatelimitingWithBurst tests that burst limit is correctly applied to an Access API call
func (suite *RateLimitTestSuite) TestRatelimitingWithBurst() {

	req := &accessproto.PingRequest{}
	ctx := context.Background()

	// expect rpc.defaultBurst number of upstream calls
	suite.execClient.On("Ping", mock.Anything, mock.Anything).Return(nil, nil).Times(suite.burstLimit)
	suite.collClient.On("Ping", mock.Anything, mock.Anything).Return(nil, nil).Times(suite.burstLimit)

	requestCnt := 0
	// generate a permissible burst of request and assert that they succeed
	for requestCnt < suite.burstLimit {
		resp, err := suite.client.Ping(ctx, req)
		assert.NoError(suite.T(), err)
		assert.NotNil(suite.T(), resp)
		requestCnt++
	}

	// request more than the permissible burst and assert that it fails
	_, err := suite.client.Ping(ctx, req)
	suite.assertRateLimitError(err)
}

func (suite *RateLimitTestSuite) assertRateLimitError(err error) {
	assert.Error(suite.T(), err)
	status, ok := status.FromError(err)
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), codes.ResourceExhausted, status.Code())
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
