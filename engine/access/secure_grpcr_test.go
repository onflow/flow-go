package access

import (
	"context"
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
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/crypto"
	accessmock "github.com/onflow/flow-go/engine/access/mock"
	"github.com/onflow/flow-go/engine/access/rpc"
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

// SecureGRPCTestSuite tests that Access node provides a secure GRPC server
type SecureGRPCTestSuite struct {
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
	rpcEng     *rpc.Engine
	publicKey  crypto.PublicKey

	// storage
	blocks       *storagemock.Blocks
	headers      *storagemock.Headers
	collections  *storagemock.Collections
	transactions *storagemock.Transactions
	receipts     *storagemock.ExecutionReceipts

	ctx    irrecoverable.SignalerContext
	cancel context.CancelFunc

	// grpc servers
	secureGrpcServer   *grpcserver.GrpcServer
	unsecureGrpcServer *grpcserver.GrpcServer
}

func (suite *SecureGRPCTestSuite) SetupTest() {
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

	config := rpc.Config{
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
	// save the public key to use later in tests later
	suite.publicKey = networkingKey.PublicKey()

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
	suite.Require().NoError(err)

	stateStreamConfig := statestreambackend.Config{}
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

	suite.rpcEng.Start(suite.ctx)

	suite.secureGrpcServer.Start(suite.ctx)
	suite.unsecureGrpcServer.Start(suite.ctx)

	// wait for the servers to startup
	unittest.AssertClosesBefore(suite.T(), suite.secureGrpcServer.Ready(), 2*time.Second)
	unittest.AssertClosesBefore(suite.T(), suite.unsecureGrpcServer.Ready(), 2*time.Second)

	// wait for the engine to startup
	unittest.AssertClosesBefore(suite.T(), suite.rpcEng.Ready(), 2*time.Second)
}

func (suite *SecureGRPCTestSuite) TearDownTest() {
	if suite.cancel != nil {
		suite.cancel()
		unittest.AssertClosesBefore(suite.T(), suite.secureGrpcServer.Done(), 2*time.Second)
		unittest.AssertClosesBefore(suite.T(), suite.unsecureGrpcServer.Done(), 2*time.Second)
		unittest.AssertClosesBefore(suite.T(), suite.rpcEng.Done(), 2*time.Second)
	}
}

func TestSecureGRPC(t *testing.T) {
	suite.Run(t, new(SecureGRPCTestSuite))
}

func (suite *SecureGRPCTestSuite) TestAPICallUsingSecureGRPC() {

	req := &accessproto.PingRequest{}
	ctx := context.Background()

	// expect 2 upstream calls
	suite.execClient.On("Ping", mock.Anything, mock.Anything).Return(nil, nil).Twice()
	suite.collClient.On("Ping", mock.Anything, mock.Anything).Return(nil, nil).Twice()

	suite.Run("happy path - grpc client can connect successfully with the correct public key", func() {
		client, closer := suite.secureGRPCClient(suite.publicKey)
		defer closer.Close()
		resp, err := client.Ping(ctx, req)
		assert.NoError(suite.T(), err)
		assert.NotNil(suite.T(), resp)
	})

	suite.Run("happy path - connection fails with an incorrect public key", func() {
		newKey := unittest.NetworkingPrivKeyFixture()
		client, closer := suite.secureGRPCClient(newKey.PublicKey())
		defer closer.Close()
		_, err := client.Ping(ctx, req)
		assert.Error(suite.T(), err)
	})

	suite.Run("happy path - connection fails, unsecure client can not get info from secure server connection", func() {
		conn, err := grpc.Dial(
			suite.secureGrpcServer.GRPCAddress().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
		assert.NoError(suite.T(), err)

		client := accessproto.NewAccessAPIClient(conn)
		closer := io.Closer(conn)
		defer closer.Close()

		_, err = client.Ping(ctx, req)
		assert.Error(suite.T(), err)
	})
}

// secureGRPCClient creates a secure GRPC client using the given public key
func (suite *SecureGRPCTestSuite) secureGRPCClient(publicKey crypto.PublicKey) (accessproto.AccessAPIClient, io.Closer) {
	tlsConfig, err := grpcutils.DefaultClientTLSConfig(publicKey)
	assert.NoError(suite.T(), err)

	conn, err := grpc.Dial(
		suite.secureGrpcServer.GRPCAddress().String(),
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	assert.NoError(suite.T(), err)

	client := accessproto.NewAccessAPIClient(conn)
	closer := io.Closer(conn)
	return client, closer
}
