package access

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/antihax/optional"
	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	restclient "github.com/onflow/flow/openapi/go-client-generated"

	"github.com/onflow/crypto"
	accessmock "github.com/onflow/flow-go/engine/access/mock"
	"github.com/onflow/flow-go/engine/access/rest"
	"github.com/onflow/flow-go/engine/access/rest/routes"
	"github.com/onflow/flow-go/engine/access/rpc"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	statestreambackend "github.com/onflow/flow-go/engine/access/state_stream/backend"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/grpcserver"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/mocknetwork"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/grpcutils"
	"github.com/onflow/flow-go/utils/unittest"
)

// IrrecoverableStateTestSuite tests that Access node indicate an inconsistent or corrupted node state
type IrrecoverableStateTestSuite struct {
	suite.Suite
	state      *protocol.State
	snapshot   *protocol.Snapshot
	epochQuery *protocol.EpochQuery
	log        zerolog.Logger
	net        *mocknetwork.EngineRegistry
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

	ctx irrecoverable.SignalerContext

	// grpc servers
	secureGrpcServer   *grpcserver.GrpcServer
	unsecureGrpcServer *grpcserver.GrpcServer
}

func (suite *IrrecoverableStateTestSuite) SetupTest() {
	suite.log = zerolog.New(os.Stdout)
	suite.net = mocknetwork.NewEngineRegistry(suite.T())
	suite.state = protocol.NewState(suite.T())
	suite.snapshot = protocol.NewSnapshot(suite.T())

	rootHeader := unittest.BlockHeaderFixture()
	params := protocol.NewParams(suite.T())
	params.On("SporkID").Return(unittest.IdentifierFixture(), nil)
	params.On("ProtocolVersion").Return(uint(unittest.Uint64InRange(10, 30)), nil)
	params.On("SporkRootBlockHeight").Return(rootHeader.Height, nil)
	params.On("SealedRoot").Return(rootHeader, nil)

	suite.epochQuery = protocol.NewEpochQuery(suite.T())
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.state.On("Params").Return(params, nil).Maybe()
	suite.snapshot.On("Epochs").Return(suite.epochQuery).Maybe()
	suite.blocks = storagemock.NewBlocks(suite.T())
	suite.headers = storagemock.NewHeaders(suite.T())
	suite.transactions = storagemock.NewTransactions(suite.T())
	suite.collections = storagemock.NewCollections(suite.T())
	suite.receipts = storagemock.NewExecutionReceipts(suite.T())

	suite.collClient = accessmock.NewAccessAPIClient(suite.T())
	suite.execClient = accessmock.NewExecutionAPIClient(suite.T())

	suite.request = module.NewRequester(suite.T())
	suite.me = module.NewLocal(suite.T())

	accessIdentity := unittest.IdentityFixture(unittest.WithRole(flow.RoleAccess))
	suite.me.
		On("NodeID").
		Return(accessIdentity.NodeID).Maybe()

	suite.chainID = flow.Testnet
	suite.metrics = metrics.NewNoopCollector()

	config := rpc.Config{
		UnsecureGRPCListenAddr: unittest.DefaultAddress,
		SecureGRPCListenAddr:   unittest.DefaultAddress,
		HTTPListenAddr:         unittest.DefaultAddress,
		RestConfig: rest.Config{
			ListenAddress: unittest.DefaultAddress,
		},
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

	blockHeader := unittest.BlockHeaderFixture()
	suite.snapshot.On("Head").Return(blockHeader, nil).Once()

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

	err = fmt.Errorf("inconsistent node's state")
	signCtxErr := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
	ctx := irrecoverable.NewMockSignalerContextExpectError(suite.T(), context.Background(), signCtxErr)

	suite.rpcEng.Start(ctx)

	suite.secureGrpcServer.Start(ctx)
	suite.unsecureGrpcServer.Start(ctx)

	// wait for the servers to startup
	unittest.AssertClosesBefore(suite.T(), suite.secureGrpcServer.Ready(), 2*time.Second)
	unittest.AssertClosesBefore(suite.T(), suite.unsecureGrpcServer.Ready(), 2*time.Second)

	// wait for the engine to startup
	unittest.AssertClosesBefore(suite.T(), suite.rpcEng.Ready(), 2*time.Second)
}

func TestIrrecoverableState(t *testing.T) {
	suite.Run(t, new(IrrecoverableStateTestSuite))
}

// TestGRPCInconsistentNodeState tests the behavior when gRPC encounters an inconsistent node state.
func (suite *IrrecoverableStateTestSuite) TestGRPCInconsistentNodeState() {
	err := fmt.Errorf("inconsistent node's state")
	suite.snapshot.On("Head").Return(nil, err)

	conn, err := grpc.Dial(
		suite.unsecureGrpcServer.GRPCAddress().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NoError(suite.T(), err)
	defer io.Closer(conn).Close()

	client := accessproto.NewAccessAPIClient(conn)

	req := &accessproto.GetAccountAtLatestBlockRequest{
		Address: unittest.AddressFixture().Bytes(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	actual, err := client.GetAccountAtLatestBlock(ctx, req)
	suite.Require().Error(err)
	suite.Require().Nil(actual)
}

// TestRestInconsistentNodeState tests the behavior when the REST API encounters an inconsistent node state.
func (suite *IrrecoverableStateTestSuite) TestRestInconsistentNodeState() {
	collections := unittest.CollectionListFixture(1)
	blockHeader := unittest.BlockWithGuaranteesFixture(
		unittest.CollectionGuaranteesWithCollectionIDFixture(collections),
	)
	suite.blocks.On("ByID", blockHeader.ID()).Return(blockHeader, nil)

	err := fmt.Errorf("inconsistent node's state")
	suite.snapshot.On("Head").Return(nil, err)

	config := restclient.NewConfiguration()
	config.BasePath = fmt.Sprintf("http://%s/v1", suite.rpcEng.RestApiAddress().String())
	client := restclient.NewAPIClient(config)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	actual, _, err := client.BlocksApi.BlocksIdGet(ctx, []string{blockHeader.ID().String()}, optionsForBlocksIdGetOpts())
	suite.Require().Error(err)
	suite.Require().Nil(actual)
}

// optionsForBlocksIdGetOpts returns options for the BlocksApi.BlocksIdGet function.
func optionsForBlocksIdGetOpts() *restclient.BlocksApiBlocksIdGetOpts {
	return &restclient.BlocksApiBlocksIdGetOpts{
		Expand:  optional.NewInterface([]string{routes.ExpandableFieldPayload}),
		Select_: optional.NewInterface([]string{"header.id"}),
	}
}
