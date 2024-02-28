package access

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	executiondataproto "github.com/onflow/flow/protobuf/go/flow/executiondata"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/index"
	accessmock "github.com/onflow/flow-go/engine/access/mock"
	"github.com/onflow/flow-go/engine/access/rpc"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	statestreambackend "github.com/onflow/flow-go/engine/access/state_stream/backend"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	"github.com/onflow/flow-go/module/grpcserver"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/herocache"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/grpcutils"
	"github.com/onflow/flow-go/utils/unittest"
)

// SameGRPCPortTestSuite verifies both AccessAPI and ExecutionDataAPI client continue to work when configured
// on the same port
type SameGRPCPortTestSuite struct {
	suite.Suite
	state                *protocol.State
	snapshot             *protocol.Snapshot
	epochQuery           *protocol.EpochQuery
	log                  zerolog.Logger
	net                  *network.EngineRegistry
	request              *module.Requester
	collClient           *accessmock.AccessAPIClient
	execClient           *accessmock.ExecutionAPIClient
	me                   *module.Local
	chainID              flow.ChainID
	metrics              *metrics.NoopCollector
	rpcEng               *rpc.Engine
	stateStreamEng       *statestreambackend.Engine
	executionDataTracker subscription.ExecutionDataTracker

	// storage
	blocks       *storagemock.Blocks
	headers      *storagemock.Headers
	events       *storagemock.Events
	collections  *storagemock.Collections
	transactions *storagemock.Transactions
	receipts     *storagemock.ExecutionReceipts
	seals        *storagemock.Seals
	results      *storagemock.ExecutionResults
	registers    *execution.RegistersAsyncStore

	ctx    irrecoverable.SignalerContext
	cancel context.CancelFunc

	// grpc servers
	secureGrpcServer   *grpcserver.GrpcServer
	unsecureGrpcServer *grpcserver.GrpcServer

	bs                blobs.Blobstore
	eds               execution_data.ExecutionDataStore
	broadcaster       *engine.Broadcaster
	execDataCache     *cache.ExecutionDataCache
	execDataHeroCache *herocache.BlockExecutionData

	blockMap map[uint64]*flow.Block
}

func (suite *SameGRPCPortTestSuite) SetupTest() {
	suite.log = zerolog.New(os.Stdout)
	suite.net = new(network.EngineRegistry)
	suite.state = new(protocol.State)
	suite.snapshot = new(protocol.Snapshot)
	params := new(protocol.Params)
	suite.registers = execution.NewRegistersAsyncStore()

	suite.epochQuery = new(protocol.EpochQuery)
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.state.On("Params").Return(params)
	suite.snapshot.On("Epochs").Return(suite.epochQuery).Maybe()
	suite.blocks = new(storagemock.Blocks)
	suite.headers = new(storagemock.Headers)
	suite.events = new(storagemock.Events)
	suite.transactions = new(storagemock.Transactions)
	suite.collections = new(storagemock.Collections)
	suite.receipts = new(storagemock.ExecutionReceipts)
	suite.results = new(storagemock.ExecutionResults)
	suite.seals = new(storagemock.Seals)

	suite.collClient = new(accessmock.AccessAPIClient)
	suite.execClient = new(accessmock.ExecutionAPIClient)

	suite.request = new(module.Requester)
	suite.request.On("EntityByID", mock.Anything, mock.Anything)

	suite.me = new(module.Local)
	suite.eds = execution_data.NewExecutionDataStore(suite.bs, execution_data.DefaultSerializer)

	suite.broadcaster = engine.NewBroadcaster()

	suite.execDataHeroCache = herocache.NewBlockExecutionData(subscription.DefaultCacheSize, suite.log, metrics.NewNoopCollector())
	suite.execDataCache = cache.NewExecutionDataCache(suite.eds, suite.headers, suite.seals, suite.results, suite.execDataHeroCache)

	accessIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleAccess))
	suite.me.
		On("NodeID").
		Return(accessIdentity.NodeID)

	suite.chainID = flow.Testnet
	suite.metrics = metrics.NewNoopCollector()

	config := rpc.Config{
		UnsecureGRPCListenAddr: unittest.DefaultAddress,
		SecureGRPCListenAddr:   unittest.DefaultAddress,
		HTTPListenAddr:         unittest.DefaultAddress,
	}

	blockCount := 5
	suite.blockMap = make(map[uint64]*flow.Block, blockCount)
	// generate blockCount consecutive blocks with associated seal, result and execution data
	rootBlock := unittest.BlockFixture()
	parent := rootBlock.Header
	suite.blockMap[rootBlock.Header.Height] = &rootBlock

	for i := 0; i < blockCount; i++ {
		block := unittest.BlockWithParentFixture(parent)
		suite.blockMap[block.Header.Height] = block
	}

	params.On("SporkID").Return(unittest.IdentifierFixture(), nil)
	params.On("ProtocolVersion").Return(uint(unittest.Uint64InRange(10, 30)), nil)
	params.On("SporkRootBlockHeight").Return(rootBlock.Header.Height, nil)
	params.On("SealedRoot").Return(rootBlock.Header, nil)

	// generate a server certificate that will be served by the GRPC server
	networkingKey := unittest.NetworkingPrivKeyFixture()
	x509Certificate, err := grpcutils.X509Certificate(networkingKey)
	assert.NoError(suite.T(), err)
	tlsConfig := grpcutils.DefaultServerTLSConfig(x509Certificate)
	// set the transport credentials for the server to use
	config.TransportCredentials = credentials.NewTLS(tlsConfig)

	suite.secureGrpcServer = grpcserver.NewGrpcServerBuilder(suite.log,
		config.SecureGRPCListenAddr,
		grpcutils.DefaultMaxMsgSize,
		false,
		nil,
		nil,
		grpcserver.WithTransportCredentials(config.TransportCredentials)).Build()

	suite.unsecureGrpcServer = grpcserver.NewGrpcServerBuilder(suite.log,
		config.UnsecureGRPCListenAddr,
		grpcutils.DefaultMaxMsgSize,
		false,
		nil,
		nil).Build()

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
	require.NoError(suite.T(), err)

	stateStreamConfig := statestreambackend.Config{}
	// create rpc engine builder
	rpcEngBuilder, err := rpc.NewBuilder(
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
		stateStreamConfig,
	)
	assert.NoError(suite.T(), err)
	suite.rpcEng, err = rpcEngBuilder.WithLegacy().Build()
	assert.NoError(suite.T(), err)
	suite.ctx, suite.cancel = irrecoverable.NewMockSignalerContextWithCancel(suite.T(), context.Background())

	suite.headers.On("BlockIDByHeight", mock.AnythingOfType("uint64")).Return(
		func(height uint64) flow.Identifier {
			if block, ok := suite.blockMap[height]; ok {
				return block.Header.ID()
			}
			return flow.ZeroID
		},
		func(height uint64) error {
			if _, ok := suite.blockMap[height]; ok {
				return nil
			}
			return storage.ErrNotFound
		},
	).Maybe()

	conf := statestreambackend.Config{
		ClientSendTimeout:    subscription.DefaultSendTimeout,
		ClientSendBufferSize: subscription.DefaultSendBufferSize,
	}

	eventIndexer := index.NewEventsIndex(suite.events)

	suite.executionDataTracker, err = subscription.NewExecutionDataTracker(
		suite.state,
		rootBlock.Header.Height,
		suite.headers,
		rootBlock.Header.Height,
		eventIndexer,
		false,
	)
	require.NoError(suite.T(), err)

	stateStreamBackend, err := statestreambackend.New(
		suite.log,
		conf,
		suite.state,
		suite.headers,
		suite.seals,
		suite.results,
		nil,
		suite.execDataCache,
		nil,
		suite.registers,
		eventIndexer,
		false,
		suite.executionDataTracker,
	)
	assert.NoError(suite.T(), err)

	// create state stream engine
	suite.stateStreamEng, err = statestreambackend.NewEng(
		suite.log,
		conf,
		suite.execDataCache,
		suite.headers,
		suite.chainID,
		suite.unsecureGrpcServer,
		stateStreamBackend,
		nil,
	)
	assert.NoError(suite.T(), err)

	suite.rpcEng.Start(suite.ctx)
	suite.stateStreamEng.Start(suite.ctx)

	suite.secureGrpcServer.Start(suite.ctx)
	suite.unsecureGrpcServer.Start(suite.ctx)

	// wait for the servers to startup
	unittest.AssertClosesBefore(suite.T(), suite.secureGrpcServer.Ready(), 2*time.Second)
	unittest.AssertClosesBefore(suite.T(), suite.unsecureGrpcServer.Ready(), 2*time.Second)

	// wait for the rpc engine to startup
	unittest.AssertClosesBefore(suite.T(), suite.rpcEng.Ready(), 2*time.Second)
	// wait for the state stream engine to startup
	unittest.AssertClosesBefore(suite.T(), suite.stateStreamEng.Ready(), 2*time.Second)
}

// TestEnginesOnTheSameGrpcPort verifies if both AccessAPI and ExecutionDataAPI client successfully connect and continue
// to work when configured on the same port
func (suite *SameGRPCPortTestSuite) TestEnginesOnTheSameGrpcPort() {
	ctx := context.Background()

	conn, err := grpc.Dial(
		suite.unsecureGrpcServer.GRPCAddress().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NoError(suite.T(), err)
	closer := io.Closer(conn)

	suite.Run("happy path - grpc access api client can connect successfully", func() {
		req := &accessproto.GetNetworkParametersRequest{}

		// expect 2 upstream calls
		suite.execClient.On("GetNetworkParameters", mock.Anything, mock.Anything).Return(nil, nil).Twice()
		suite.collClient.On("GetNetworkParameters", mock.Anything, mock.Anything).Return(nil, nil).Twice()

		client := suite.unsecureAccessAPIClient(conn)

		_, err := client.GetNetworkParameters(ctx, req)
		assert.NoError(suite.T(), err, "failed to get network")
	})

	suite.Run("happy path - grpc execution data api client can connect successfully", func() {
		req := &executiondataproto.SubscribeEventsRequest{}

		client := suite.unsecureExecutionDataAPIClient(conn)

		_, err := client.SubscribeEvents(ctx, req)
		assert.NoError(suite.T(), err, "failed to subscribe events")
	})
	defer closer.Close()
}

func TestSameGRPCTestSuite(t *testing.T) {
	suite.Run(t, new(SameGRPCPortTestSuite))
}

// unsecureAccessAPIClient creates an unsecure grpc AccessAPI client
func (suite *SameGRPCPortTestSuite) unsecureAccessAPIClient(conn *grpc.ClientConn) accessproto.AccessAPIClient {
	client := accessproto.NewAccessAPIClient(conn)
	return client
}

// unsecureExecutionDataAPIClient creates an unsecure ExecutionDataAPI client
func (suite *SameGRPCPortTestSuite) unsecureExecutionDataAPIClient(conn *grpc.ClientConn) executiondataproto.ExecutionDataAPIClient {
	client := executiondataproto.NewExecutionDataAPIClient(conn)
	return client
}
