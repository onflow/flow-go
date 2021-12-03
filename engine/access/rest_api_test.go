package access

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/antihax/optional"
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
// The tests starts the REST server locally and uses the Go REST client on onflow/flow repo to make the api requests.
// The server uses storage mocks
type RestAPITestSuite struct {
	suite.Suite
	state             *protocol.State
	sealedSnaphost    *protocol.Snapshot
	finalizedSnapshot *protocol.Snapshot
	log               zerolog.Logger
	net               *network.Network
	request           *module.Requester
	collClient        *accessmock.AccessAPIClient
	execClient        *accessmock.ExecutionAPIClient
	me                *module.Local
	chainID           flow.ChainID
	metrics           *metrics.NoopCollector
	rpcEng            *rpc.Engine

	// storage
	blocks           *storagemock.Blocks
	headers          *storagemock.Headers
	collections      *storagemock.Collections
	transactions     *storagemock.Transactions
	receipts         *storagemock.ExecutionReceipts
	executionResults *storagemock.ExecutionResults
}

func (suite *RestAPITestSuite) SetupTest() {
	suite.log = zerolog.New(os.Stdout)
	suite.net = new(network.Network)
	suite.state = new(protocol.State)
	suite.sealedSnaphost = new(protocol.Snapshot)
	suite.finalizedSnapshot = new(protocol.Snapshot)

	suite.state.On("Sealed").Return(suite.sealedSnaphost, nil)
	suite.state.On("Final").Return(suite.finalizedSnapshot, nil)
	suite.blocks = new(storagemock.Blocks)
	suite.headers = new(storagemock.Headers)
	suite.transactions = new(storagemock.Transactions)
	suite.collections = new(storagemock.Collections)
	suite.receipts = new(storagemock.ExecutionReceipts)
	suite.executionResults = new(storagemock.ExecutionResults)

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
		nil, suite.executionResults, suite.chainID, suite.metrics, 0, 0, false, false, nil, nil)
	unittest.AssertClosesBefore(suite.T(), suite.rpcEng.Ready(), 2*time.Second)

	// wait for the server to startup
	assert.Eventually(suite.T(), func() bool {
		return suite.rpcEng.RestApiAddress() != nil
	}, 5*time.Second, 10*time.Millisecond)
}

func TestRestAPI(t *testing.T) {
	suite.Run(t, new(RestAPITestSuite))
}

func (suite *RestAPITestSuite) TestGetBlock() {

	testBlockIDs := make([]string, rest.MaxAllowedBlockIDs)
	testBlocks := make([]*flow.Block, rest.MaxAllowedBlockIDs)
	for i := range testBlockIDs {
		collections := unittest.CollectionListFixture(1)
		block := unittest.BlockWithGuaranteesFixture(
			unittest.CollectionGuaranteesWithCollectionIDFixture(collections),
		)
		block.Header.Height = uint64(i)
		suite.blocks.On("ByID", block.ID()).Return(block, nil)
		suite.blocks.On("ByHeight", block.Header.Height).Return(block, nil)
		testBlocks[i] = block
		testBlockIDs[i] = block.ID().String()

		execResult := unittest.ExecutionResultFixture()
		suite.executionResults.On("ByBlockID", block.ID()).Return(execResult, nil)
	}

	sealedBlock := testBlocks[len(testBlocks)-1]
	finalizedBlock := testBlocks[len(testBlocks)-2]
	suite.sealedSnaphost.On("Head").Return(sealedBlock.Header, nil)
	suite.finalizedSnapshot.On("Head").Return(finalizedBlock.Header, nil)

	client := suite.restAPIClient()

	suite.Run("GetBlockByID for a single ID - happy path", func() {

		testBlock := testBlocks[0]
		suite.blocks.On("ByID", testBlock.ID()).Return(testBlock, nil).Once()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		respBlocks, resp, err := client.BlocksApi.BlocksIdGet(ctx, []string{testBlock.ID().String()}, optionsForBlockByID())
		require.NoError(suite.T(), err)
		require.Equal(suite.T(), http.StatusOK, resp.StatusCode)
		require.Len(suite.T(), respBlocks, 1)
		assert.Equal(suite.T(), testBlock.ID().String(), testBlock.ID().String())

		require.Nil(suite.T(), respBlocks[0].ExecutionResult)
	})

	suite.Run("GetBlockByID for multiple IDs - happy path", func() {

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		// the swagger generated Go client code has bug where it generates a space delimited list of ids instead of a
		// comma delimited one. hence, explicitly setting the ids as a csv here
		blockIDSlice := []string{strings.Join(testBlockIDs, ",")}

		actualBlocks, resp, err := client.BlocksApi.BlocksIdGet(ctx, blockIDSlice, optionsForBlockByID())
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
		assert.Len(suite.T(), actualBlocks, rest.MaxAllowedBlockIDs)
		for i, b := range testBlocks {
			assert.Equal(suite.T(), b.ID().String(), actualBlocks[i].Header.Id)
		}
	})

	suite.Run("GetBlockByHeight by start and end height - happy path", func() {

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		startHeight := testBlocks[0].Header.Height
		blkCnt := len(testBlocks)
		endHeight := testBlocks[blkCnt-1].Header.Height

		actualBlocks, resp, err := client.BlocksApi.BlocksGet(ctx, optionsForBlockByStartEndHeight(startHeight, endHeight))
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
		assert.Len(suite.T(), actualBlocks, blkCnt)
		for i := 0; i < blkCnt; i++ {
			assert.Equal(suite.T(), testBlocks[i].ID().String(), actualBlocks[i].Header.Id)
			assert.Equal(suite.T(), fmt.Sprintf("%d", testBlocks[i].Header.Height), actualBlocks[i].Header.Height)
		}
	})

	suite.Run("GetBlockByHeight by heights - happy path", func() {

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		lastIndex := len(testBlocks)
		var reqHeights = make([]uint64, len(testBlocks))
		for i := 0; i < lastIndex; i++ {
			reqHeights[i] = testBlocks[i].Header.Height
		}

		actualBlocks, resp, err := client.BlocksApi.BlocksGet(ctx, optionsForBlockByHeights(reqHeights))
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
		assert.Len(suite.T(), actualBlocks, lastIndex)
		for i := 0; i < lastIndex; i++ {
			assert.Equal(suite.T(), testBlocks[i].ID().String(), actualBlocks[i].Header.Id)
			assert.Equal(suite.T(), fmt.Sprintf("%d", testBlocks[i].Header.Height), actualBlocks[i].Header.Height)
		}
	})

	suite.Run("GetBlockByHeight for height=final - happy path", func() {

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		actualBlocks, resp, err := client.BlocksApi.BlocksGet(ctx, optionsForFinalizedBlock("final"))
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
		assert.Len(suite.T(), actualBlocks, 1)
		assert.Equal(suite.T(), finalizedBlock.ID().String(), actualBlocks[0].Header.Id)
	})

	suite.Run("GetBlockByHeight for height=sealed happy path", func() {

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		actualBlocks, resp, err := client.BlocksApi.BlocksGet(ctx, optionsForFinalizedBlock("sealed"))
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
		assert.Len(suite.T(), actualBlocks, 1)
		assert.Equal(suite.T(), sealedBlock.ID().String(), actualBlocks[0].Header.Id)
	})

	suite.Run("GetBlockByID with a non-existing block ID", func() {

		nonExistingBlockID := unittest.IdentifierFixture()
		suite.blocks.On("ByID", nonExistingBlockID).Return(nil, storage.ErrNotFound).Once()

		client := suite.restAPIClient()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		_, resp, err := client.BlocksApi.BlocksIdGet(ctx, []string{nonExistingBlockID.String()}, optionsForBlockByID())
		assertError(suite.T(), resp, err, http.StatusNotFound, fmt.Sprintf("block with ID %s not found", nonExistingBlockID.String()))
	})

	suite.Run("GetBlockByID with an invalid block ID", func() {

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		const invalidBlockID = "invalid_block_id"
		_, resp, err := client.BlocksApi.BlocksIdGet(ctx, []string{invalidBlockID}, optionsForBlockByID())
		assertError(suite.T(), resp, err, http.StatusBadRequest, fmt.Sprintf("invalid ID %s", invalidBlockID))
	})

	suite.Run("GetBlockByID with more than the permissible number of block IDs", func() {

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		blockIDs := make([]string, rest.MaxAllowedBlockIDs+1)
		copy(blockIDs, testBlockIDs)
		blockIDs[rest.MaxAllowedBlockIDs] = unittest.IdentifierFixture().String()

		blockIDSlice := []string{strings.Join(blockIDs, ",")}
		_, resp, err := client.BlocksApi.BlocksIdGet(ctx, blockIDSlice, optionsForBlockByID())
		assertError(suite.T(), resp, err, http.StatusBadRequest, fmt.Sprintf("at most %d Block IDs can be requested at a time", rest.MaxAllowedBlockIDs))
	})

	suite.Run("GetBlockByID with one non-existing block ID", func() {

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		// replace one ID with a block ID for which the storage returns a not found error
		rand.Seed(time.Now().Unix())
		invalidBlockIndex := rand.Intn(len(testBlocks))
		invalidID := unittest.IdentifierFixture()
		suite.blocks.On("ByID", invalidID).Return(nil, storage.ErrNotFound).Once()
		blockIDs := make([]string, len(testBlockIDs))
		copy(blockIDs, testBlockIDs)
		blockIDs[invalidBlockIndex] = invalidID.String()

		blockIDSlice := []string{strings.Join(blockIDs, ",")}
		_, resp, err := client.BlocksApi.BlocksIdGet(ctx, blockIDSlice, optionsForBlockByID())
		assertError(suite.T(), resp, err, http.StatusNotFound, fmt.Sprintf("block with ID %s not found", blockIDs[invalidBlockIndex]))
	})

	suite.Run("GetBlockByHeight by non-existing height", func() {

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		invalidHeight := uint64(len(testBlocks))
		var reqHeights = []uint64{invalidHeight}
		suite.blocks.On("ByHeight", invalidHeight).Return(nil, storage.ErrNotFound).Once()

		_, resp, err := client.BlocksApi.BlocksGet(ctx, optionsForBlockByHeights(reqHeights))
		assertError(suite.T(), resp, err, http.StatusNotFound, fmt.Sprintf("block at height %d not found", invalidHeight))
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

func optionsForBlockByID() *restclient.BlocksApiBlocksIdGetOpts {
	return &restclient.BlocksApiBlocksIdGetOpts{
		Expand:  optional.NewInterface([]string{rest.ExpandableFieldPayload}),
		Select_: optional.NewInterface([]string{"header.id"}),
	}
}
func optionsForBlockByStartEndHeight(startHeight, endHeight uint64) *restclient.BlocksApiBlocksGetOpts {
	return &restclient.BlocksApiBlocksGetOpts{
		Expand:      optional.NewInterface([]string{rest.ExpandableFieldPayload}),
		Select_:     optional.NewInterface([]string{"header.id", "header.height"}),
		StartHeight: optional.NewInterface(startHeight),
		EndHeight:   optional.NewInterface(endHeight),
	}
}

func optionsForBlockByHeights(heights []uint64) *restclient.BlocksApiBlocksGetOpts {
	return &restclient.BlocksApiBlocksGetOpts{
		Expand:  optional.NewInterface([]string{rest.ExpandableFieldPayload}),
		Select_: optional.NewInterface([]string{"header.id", "header.height"}),
		Height:  optional.NewInterface(heights),
	}
}

func optionsForFinalizedBlock(finalOrSealed string) *restclient.BlocksApiBlocksGetOpts {
	return &restclient.BlocksApiBlocksGetOpts{
		Expand:  optional.NewInterface([]string{rest.ExpandableFieldPayload}),
		Select_: optional.NewInterface([]string{"header.id", "header.height"}),
		Height:  optional.NewInterface(finalOrSealed),
	}
}
