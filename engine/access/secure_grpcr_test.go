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

	"github.com/onflow/flow-go/crypto"
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

// SecureGRPCTestSuite tests that Access node provides a secure GRPC server
type SecureGRPCTestSuite struct {
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
	publicKey  crypto.PublicKey

	// storage
	blocks       *storagemock.Blocks
	headers      *storagemock.Headers
	collections  *storagemock.Collections
	transactions *storagemock.Transactions
	receipts     *storagemock.ExecutionReceipts
}

func (suite *SecureGRPCTestSuite) SetupTest() {
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

	// generate a server certificate that will be served by the GRPC server
	networkingKey := unittest.NetworkingPrivKeyFixture()
	x509Certificate, err := grpcutils.X509Certificate(networkingKey)
	assert.NoError(suite.T(), err)
	tlsConfig := grpcutils.DefaultServerTLSConfig(x509Certificate)
	// set the transport credentials for the server to use
	config.TransportCredentials = credentials.NewTLS(tlsConfig)
	// save the public key to use later in tests later
	suite.publicKey = networkingKey.PublicKey()

	suite.rpcEng = rpc.New(suite.log, suite.state, config, suite.collClient, nil, suite.blocks, suite.headers, suite.collections, suite.transactions,
		nil, nil, suite.chainID, suite.metrics, 0, 0, false, false, nil, nil)
	unittest.AssertClosesBefore(suite.T(), suite.rpcEng.Ready(), 2*time.Second)

	// wait for the server to startup
	assert.Eventually(suite.T(), func() bool {
		return suite.rpcEng.SecureGRPCAddress() != nil
	}, 5*time.Second, 10*time.Millisecond)
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
}

func (suite *SecureGRPCTestSuite) TearDownTest() {
	// close the server
	if suite.rpcEng != nil {
		unittest.AssertClosesBefore(suite.T(), suite.rpcEng.Done(), 2*time.Second)
	}
}

// secureGRPCClient creates a secure GRPC client using the given public key
func (suite *SecureGRPCTestSuite) secureGRPCClient(publicKey crypto.PublicKey) (accessproto.AccessAPIClient, io.Closer) {
	tlsConfig, err := grpcutils.DefaultClientTLSConfig(publicKey)
	assert.NoError(suite.T(), err)

	conn, err := grpc.Dial(
		suite.rpcEng.SecureGRPCAddress().String(),
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	assert.NoError(suite.T(), err)

	client := accessproto.NewAccessAPIClient(conn)
	closer := io.Closer(conn)
	return client, closer
}
