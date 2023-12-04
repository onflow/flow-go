package rpc

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
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	accessmock "github.com/onflow/flow-go/engine/access/mock"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	statestreambackend "github.com/onflow/flow-go/engine/access/state_stream/backend"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/grpcserver"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network"
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
	net        *network.EngineRegistry
	request    *module.Requester
	collClient *accessmock.AccessAPIClient
	execClient *accessmock.ExecutionAPIClient
	me         *module.Local
	chainID    flow.ChainID
	metrics    *metrics.NoopCollector
	rpcEng     *Engine
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

	ctx    irrecoverable.SignalerContext
	cancel context.CancelFunc

	// grpc servers
	secureGrpcServer   *grpcserver.GrpcServer
	unsecureGrpcServer *grpcserver.GrpcServer
}

func (suite *RateLimitTestSuite) SetupTest() {
	suite.log = zerolog.New(os.Stdout)
	suite.net = new(network.EngineRegistry)
	suite.state = new(protocol.State)
	suite.snapshot = new(protocol.Snapshot)

	rootHeader := unittest.BlockHeaderFixture()
	params := new(protocol.Params)
	params.On("SporkID").Return(unittest.IdentifierFixture(), nil)
	params.On("ProtocolVersion").Return(uint(unittest.Uint64InRange(10, 30)), nil)
	params.On("SporkRootBlockHeight").Return(rootHeader.Height, nil)
	params.On("SealedRoot").Return(rootHeader, nil)

	suite.epochQuery = new(protocol.EpochQuery)
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.state.On("Params").Return(params, nil).Maybe()
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

	config := Config{
		UnsecureGRPCListenAddr: unittest.DefaultAddress,
		SecureGRPCListenAddr:   unittest.DefaultAddress,
		HTTPListenAddr:         unittest.DefaultAddress,
	}

	// generate a server certificate that will be served by the GRPC server
	networkingKey := unittest.NetworkingPrivKeyFixture()
	x509Certificate, err := grpcutils.X509Certificate(networkingKey)
	assert.NoError(suite.T(), err)
	tlsConfig := grpcutils.DefaultServerTLSConfig(x509Certificate)
	// set the transport credentials for the server to use
	config.TransportCredentials = credentials.NewTLS(tlsConfig)

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

	suite.secureGrpcServer = grpcserver.NewGrpcServerBuilder(suite.log,
		config.SecureGRPCListenAddr,
		grpcutils.DefaultMaxMsgSize,
		false,
		apiRateLimt,
		apiBurstLimt,
		grpcserver.WithTransportCredentials(config.TransportCredentials)).Build()

	suite.unsecureGrpcServer = grpcserver.NewGrpcServerBuilder(suite.log,
		config.UnsecureGRPCListenAddr,
		grpcutils.DefaultMaxMsgSize,
		false,
		apiRateLimt,
		apiBurstLimt).Build()

	block := unittest.BlockHeaderFixture()
	suite.snapshot.On("Head").Return(block, nil)

	bnd, err := backend.New(backend.Params{
		State:                suite.state,
		CollectionRPC:        suite.collClient,
		Blocks:               suite.blocks,
		Headers:              suite.headers,
		Collections:          suite.collections,
		Transactions:         suite.transactions,
		ChainID:              suite.chainID,
		AccessMetrics:        suite.metrics,
		MaxHeightRange:       0,
		Log:                  suite.log,
		SnapshotHistoryLimit: 0,
		Communicator:         backend.NewNodeCommunicator(false),
	})
	suite.Require().NoError(err)

	stateStreamConfig := statestreambackend.Config{}
	rpcEngBuilder, err := NewBuilder(
		suite.log,
		suite.state,
		config,
		suite.chainID,
		suite.metrics,
		false,
		suite.me,
		bnd,
		bnd,
		suite.secureGrpcServer,
		suite.unsecureGrpcServer,
		nil,
		stateStreamConfig)
	require.NoError(suite.T(), err)
	suite.rpcEng, err = rpcEngBuilder.WithLegacy().Build()
	require.NoError(suite.T(), err)
	suite.ctx, suite.cancel = irrecoverable.NewMockSignalerContextWithCancel(suite.T(), context.Background())

	suite.rpcEng.Start(suite.ctx)

	suite.secureGrpcServer.Start(suite.ctx)
	suite.unsecureGrpcServer.Start(suite.ctx)

	// wait for the servers to startup
	unittest.AssertClosesBefore(suite.T(), suite.secureGrpcServer.Ready(), 2*time.Second)
	unittest.AssertClosesBefore(suite.T(), suite.unsecureGrpcServer.Ready(), 2*time.Second)

	// wait for the engine to startup
	unittest.RequireCloseBefore(suite.T(), suite.rpcEng.Ready(), 2*time.Second, "engine not ready at startup")

	// create the access api client
	suite.client, suite.closer, err = accessAPIClient(suite.unsecureGrpcServer.GRPCAddress().String())
	require.NoError(suite.T(), err)
}

func (suite *RateLimitTestSuite) TearDownTest() {
	if suite.cancel != nil {
		suite.cancel()
	}
	// close the client
	if suite.closer != nil {
		suite.closer.Close()
	}
	// close servers
	unittest.AssertClosesBefore(suite.T(), suite.secureGrpcServer.Done(), 2*time.Second)
	unittest.AssertClosesBefore(suite.T(), suite.unsecureGrpcServer.Done(), 2*time.Second)
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
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to address %s: %w", address, err)
	}
	client := accessproto.NewAccessAPIClient(conn)
	closer := io.Closer(conn)
	return client, closer, nil
}
