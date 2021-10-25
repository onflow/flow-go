package access

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	restclient "github.com/onflow/flow/openapi/go-client-generated"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/crypto"
	accessmock "github.com/onflow/flow-go/engine/access/mock"
	"github.com/onflow/flow-go/engine/access/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// RestAPITestSuite tests that Access node provides a secure GRPC server
type RestAPITestSuite struct {
	suite.Suite
	state      *protocol.State
	snapshot   *protocol.Snapshot
	epochQuery *protocol.EpochQuery
	log        zerolog.Logger
	net        *network.Network
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

func (suite *RestAPITestSuite) SetupTest() {
	suite.log = zerolog.New(os.Stdout)
	suite.net = new(network.Network)
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

	const anyPort = ":0" // :0 to let the OS pick a free port
	config := rpc.Config{
		UnsecureGRPCListenAddr: anyPort,
		SecureGRPCListenAddr:   anyPort,
		HTTPListenAddr:         anyPort,
		RESTListenAddr:         anyPort,
	}

	suite.rpcEng = rpc.New(suite.log, suite.state, config, suite.collClient, nil, suite.blocks, suite.headers, suite.collections, suite.transactions,
		nil, nil, suite.chainID, suite.metrics, 0, 0, false, false, nil, nil)
	unittest.AssertClosesBefore(suite.T(), suite.rpcEng.Ready(), 2*time.Second)

	// wait for the server to startup
	assert.Eventually(suite.T(), func() bool {
		return suite.rpcEng.RestApiAddress() != nil
	}, 5*time.Second, 10*time.Millisecond)
}

func TestRestAPI(t *testing.T) {
	suite.Run(t, new(RestAPITestSuite))
}

func (suite *RestAPITestSuite) TestRestAPICall() {

	suite.Run("happy path - REST client successfully executed the GetBlockByID request", func() {

		block := unittest.BlockFixture()
		suite.blocks.On("ByID", block.ID()).Return(&block, nil).Once()

		client := suite.restAPIClient()
		ctx, cancel := context.WithTimeout(context.Background(), time.Hour*5)
		defer cancel()

		blocks, resp, err := client.BlocksApi.BlocksIdGet(ctx, []string{block.ID().String()}, nil)
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
		assert.Len(suite.T(), blocks, 1)
		assert.Equal(suite.T(), block.ID().String(), blocks[0].Header.Id)
	})
}

func (suite *RestAPITestSuite) TearDownTest() {
	// close the server
	if suite.rpcEng != nil {
		unittest.AssertClosesBefore(suite.T(), suite.rpcEng.Done(), 2*time.Second)
	}
}

// restAPIClient creates a REST API client
func (suite *RestAPITestSuite) restAPIClient() *restclient.APIClient {
	config := restclient.NewConfiguration()
	config.BasePath = fmt.Sprintf("http://%s/v1", suite.rpcEng.RestApiAddress().String())
	return restclient.NewAPIClient(config)
}
