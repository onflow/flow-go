package access

import (
	"context"
	"fmt"
	"math/rand"
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

	accessmock "github.com/onflow/flow-go/engine/access/mock"
	"github.com/onflow/flow-go/engine/access/rest"
	"github.com/onflow/flow-go/engine/access/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// RestAPITestSuite tests that the Access node serves the REST API defined via the OpenApi spec accurately
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

	suite.Run("GetBlockByID for a single ID - happy path", func() {

		collections := unittest.CollectionListFixture(1)
		block := unittest.BlockWithGuaranteesFixture(
			unittest.CollectionGuaranteesWithCollectionIDFixture(collections),
		)
		suite.blocks.On("ByID", block.ID()).Return(block, nil).Once()

		client := suite.restAPIClient()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		blocks, resp, err := client.BlocksApi.BlocksIdGet(ctx, []string{block.ID().String()}, nil)
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
		assert.Len(suite.T(), blocks, 1)
		assert.Equal(suite.T(), block.ID().String(), blocks[0].Header.Id)
	})

	suite.Run("GetBlockByID for multiple IDs - happy path", func() {

		client := suite.restAPIClient()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		blockIDs := make([]string, rest.MaxAllowedBlockIDsCnt)
		blocks := make([]*flow.Block, rest.MaxAllowedBlockIDsCnt)
		for i := range blockIDs {
			id := unittest.IdentifierFixture()
			blockIDs[i] = id.String()
			collections := unittest.CollectionListFixture(1)
			block := unittest.BlockWithGuaranteesFixture(
				unittest.CollectionGuaranteesWithCollectionIDFixture(collections),
			)
			blocks[i] = block
			suite.blocks.On("ByID", id).Return(block, nil).Once()
		}

		actualBlocks, resp, err := client.BlocksApi.BlocksIdGet(ctx, blockIDs, nil)
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
		assert.Len(suite.T(), blocks, rest.MaxAllowedBlockIDsCnt)
		for i, b := range blocks {
			assert.Equal(suite.T(), b.ID().String(), actualBlocks[i].Header.Id)
		}
	})

	suite.Run("GetBlockByID with a non-existing block ID", func() {

		nonExistingBlockID := unittest.IdentifierFixture()
		suite.blocks.On("ByID", nonExistingBlockID).Return(nil, storage.ErrNotFound).Once()

		client := suite.restAPIClient()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		_, resp, err := client.BlocksApi.BlocksIdGet(ctx, []string{nonExistingBlockID.String()}, nil)
		assertError(suite.T(), resp, err, http.StatusNotFound, fmt.Sprintf("block with ID %s not found", nonExistingBlockID.String()))
	})

	suite.Run("GetBlockByID with an invalid block ID", func() {

		client := suite.restAPIClient()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		const invalidBlockID = "invalid_block_id"
		_, resp, err := client.BlocksApi.BlocksIdGet(ctx, []string{invalidBlockID}, nil)
		assertError(suite.T(), resp, err, http.StatusBadRequest, fmt.Sprintf("invalid ID %s", invalidBlockID))
	})

	suite.Run("GetBlockByID with more than the permissible number of block IDs", func() {

		client := suite.restAPIClient()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		// lower the max allowed block ID count on the server for the test
		rest.MaxAllowedBlockIDsCnt = 10
		blockIDs := make([]string, rest.MaxAllowedBlockIDsCnt+1)
		for i := range blockIDs {
			blockIDs[i] = unittest.IdentifierFixture().String()
		}

		_, resp, err := client.BlocksApi.BlocksIdGet(ctx, blockIDs, nil)
		assertError(suite.T(), resp, err, http.StatusBadRequest, fmt.Sprintf("at most %d Block IDs can be requested at a time", rest.MaxAllowedBlockIDsCnt))
	})

	suite.Run("GetBlockByID with one non-existing block ID", func() {

		client := suite.restAPIClient()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		blockIDs := make([]string, rest.MaxAllowedBlockIDsCnt)
		rand.Seed(time.Now().Unix())
		invalidBlockIndex := rand.Intn(len(blockIDs))
		for i := range blockIDs {
			id := unittest.IdentifierFixture()
			blockIDs[i] = id.String()
			if i == invalidBlockIndex {
				// return a storage not found error for one of block ID in the request
				suite.blocks.On("ByID", id).Return(nil, storage.ErrNotFound).Once()
				continue
			}
			collections := unittest.CollectionListFixture(1)
			block := unittest.BlockWithGuaranteesFixture(
				unittest.CollectionGuaranteesWithCollectionIDFixture(collections),
			)
			suite.blocks.On("ByID", id).Return(block, nil).Once()
		}

		_, resp, err := client.BlocksApi.BlocksIdGet(ctx, blockIDs, nil)
		assertError(suite.T(), resp, err, http.StatusNotFound, fmt.Sprintf("block with ID %s not found", blockIDs[invalidBlockIndex]))
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

func assertError(t *testing.T, resp *http.Response, err error, expectedCode int, expectedMsgSubstr string) {
	require.NotNil(t, resp)
	assert.Equal(t, expectedCode, resp.StatusCode)
	require.Error(t, err)
	swaggerError := err.(restclient.GenericSwaggerError)
	modelError := swaggerError.Model().(restclient.ModelError)
	require.EqualValues(t, expectedCode, modelError.Code)
	require.Contains(t, modelError.Message, expectedMsgSubstr)
}
