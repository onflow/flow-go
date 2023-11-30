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
	"google.golang.org/grpc/credentials"

	accessmock "github.com/onflow/flow-go/engine/access/mock"
	"github.com/onflow/flow-go/engine/access/rest"
	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/engine/access/rest/routes"
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
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/grpcutils"
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
	net               *network.EngineRegistry
	request           *module.Requester
	collClient        *accessmock.AccessAPIClient
	execClient        *accessmock.ExecutionAPIClient
	me                *module.Local
	chainID           flow.ChainID
	metrics           *metrics.NoopCollector
	rpcEng            *rpc.Engine
	sealedBlock       *flow.Header
	finalizedBlock    *flow.Header

	// storage
	blocks           *storagemock.Blocks
	headers          *storagemock.Headers
	collections      *storagemock.Collections
	transactions     *storagemock.Transactions
	receipts         *storagemock.ExecutionReceipts
	executionResults *storagemock.ExecutionResults

	ctx    irrecoverable.SignalerContext
	cancel context.CancelFunc

	// grpc servers
	secureGrpcServer   *grpcserver.GrpcServer
	unsecureGrpcServer *grpcserver.GrpcServer
}

func (suite *RestAPITestSuite) SetupTest() {
	suite.log = zerolog.New(os.Stdout)
	suite.net = new(network.EngineRegistry)
	suite.state = new(protocol.State)
	suite.sealedSnaphost = new(protocol.Snapshot)
	suite.finalizedSnapshot = new(protocol.Snapshot)
	suite.sealedBlock = unittest.BlockHeaderFixture(unittest.WithHeaderHeight(0))
	suite.finalizedBlock = unittest.BlockHeaderWithParentFixture(suite.sealedBlock)

	rootHeader := unittest.BlockHeaderFixture()
	params := new(protocol.Params)
	params.On("SporkID").Return(unittest.IdentifierFixture(), nil)
	params.On("ProtocolVersion").Return(uint(unittest.Uint64InRange(10, 30)), nil)
	params.On("SporkRootBlockHeight").Return(rootHeader.Height, nil)
	params.On("SealedRoot").Return(rootHeader, nil)

	suite.state.On("Sealed").Return(suite.sealedSnaphost, nil)
	suite.state.On("Final").Return(suite.finalizedSnapshot, nil)
	suite.state.On("Params").Return(params)
	suite.sealedSnaphost.On("Head").Return(
		func() *flow.Header {
			return suite.sealedBlock
		},
		nil,
	).Maybe()
	suite.finalizedSnapshot.On("Head").Return(
		func() *flow.Header {
			return suite.finalizedBlock
		},
		nil,
	).Maybe()
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

	bnd, err := backend.New(backend.Params{
		State:                    suite.state,
		CollectionRPC:            suite.collClient,
		Blocks:                   suite.blocks,
		Headers:                  suite.headers,
		Collections:              suite.collections,
		Transactions:             suite.transactions,
		ExecutionResults:         suite.executionResults,
		ChainID:                  suite.chainID,
		AccessMetrics:            suite.metrics,
		MaxHeightRange:           0,
		Log:                      suite.log,
		SnapshotHistoryLimit:     0,
		Communicator:             backend.NewNodeCommunicator(false),
		TxErrorMessagesCacheSize: 1000,
	})
	require.NoError(suite.T(), err)

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

func (suite *RestAPITestSuite) TearDownTest() {
	if suite.cancel != nil {
		suite.cancel()
		unittest.AssertClosesBefore(suite.T(), suite.secureGrpcServer.Done(), 2*time.Second)
		unittest.AssertClosesBefore(suite.T(), suite.unsecureGrpcServer.Done(), 2*time.Second)
		unittest.AssertClosesBefore(suite.T(), suite.rpcEng.Done(), 2*time.Second)
	}
}

func TestRestAPI(t *testing.T) {
	suite.Run(t, new(RestAPITestSuite))
}

func (suite *RestAPITestSuite) TestGetBlock() {

	testBlockIDs := make([]string, request.MaxIDsLength)
	testBlocks := make([]*flow.Block, request.MaxIDsLength)
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

	suite.sealedBlock = testBlocks[len(testBlocks)-1].Header
	suite.finalizedBlock = testBlocks[len(testBlocks)-2].Header

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
		assert.Len(suite.T(), actualBlocks, request.MaxIDsLength)
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
		assert.Equal(suite.T(), suite.finalizedBlock.ID().String(), actualBlocks[0].Header.Id)
	})

	suite.Run("GetBlockByHeight for height=sealed happy path", func() {

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		actualBlocks, resp, err := client.BlocksApi.BlocksGet(ctx, optionsForFinalizedBlock("sealed"))
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
		assert.Len(suite.T(), actualBlocks, 1)
		assert.Equal(suite.T(), suite.sealedBlock.ID().String(), actualBlocks[0].Header.Id)
	})

	suite.Run("GetBlockByID with a non-existing block ID", func() {

		nonExistingBlockID := unittest.IdentifierFixture()
		suite.blocks.On("ByID", nonExistingBlockID).Return(nil, storage.ErrNotFound).Once()

		client := suite.restAPIClient()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		_, resp, err := client.BlocksApi.BlocksIdGet(ctx, []string{nonExistingBlockID.String()}, optionsForBlockByID())
		assertError(suite.T(), resp, err, http.StatusNotFound, fmt.Sprintf("error looking up block with ID %s", nonExistingBlockID.String()))
	})

	suite.Run("GetBlockByID with an invalid block ID", func() {

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		const invalidBlockID = "invalid_block_id"
		_, resp, err := client.BlocksApi.BlocksIdGet(ctx, []string{invalidBlockID}, optionsForBlockByID())
		assertError(suite.T(), resp, err, http.StatusBadRequest, "invalid ID format")
	})

	suite.Run("GetBlockByID with more than the permissible number of block IDs", func() {

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		blockIDs := make([]string, request.MaxIDsLength+1)
		copy(blockIDs, testBlockIDs)
		blockIDs[request.MaxIDsLength] = unittest.IdentifierFixture().String()

		blockIDSlice := []string{strings.Join(blockIDs, ",")}
		_, resp, err := client.BlocksApi.BlocksIdGet(ctx, blockIDSlice, optionsForBlockByID())
		assertError(suite.T(), resp, err, http.StatusBadRequest, fmt.Sprintf("at most %d IDs can be requested at a time", request.MaxIDsLength))
	})

	suite.Run("GetBlockByID with one non-existing block ID", func() {

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		// replace one ID with a block ID for which the storage returns a not found error
		invalidBlockIndex := rand.Intn(len(testBlocks))
		invalidID := unittest.IdentifierFixture()
		suite.blocks.On("ByID", invalidID).Return(nil, storage.ErrNotFound).Once()
		blockIDs := make([]string, len(testBlockIDs))
		copy(blockIDs, testBlockIDs)
		blockIDs[invalidBlockIndex] = invalidID.String()

		blockIDSlice := []string{strings.Join(blockIDs, ",")}
		_, resp, err := client.BlocksApi.BlocksIdGet(ctx, blockIDSlice, optionsForBlockByID())
		assertError(suite.T(), resp, err, http.StatusNotFound, fmt.Sprintf("error looking up block with ID %s", blockIDs[invalidBlockIndex]))
	})

	suite.Run("GetBlockByHeight by non-existing height", func() {

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		invalidHeight := uint64(len(testBlocks))
		var reqHeights = []uint64{invalidHeight}
		suite.blocks.On("ByHeight", invalidHeight).Return(nil, storage.ErrNotFound).Once()

		_, resp, err := client.BlocksApi.BlocksGet(ctx, optionsForBlockByHeights(reqHeights))
		assertError(suite.T(), resp, err, http.StatusNotFound, fmt.Sprintf("error looking up block at height %d", invalidHeight))
	})
}

func (suite *RestAPITestSuite) TestRequestSizeRestriction() {
	client := suite.restAPIClient()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	// make a request of size larger than the max permitted size
	requestBytes := make([]byte, routes.MaxRequestSize+1)
	script := restclient.ScriptsBody{
		Script: string(requestBytes),
	}
	_, resp, err := client.ScriptsApi.ScriptsPost(ctx, script, nil)
	assertError(suite.T(), resp, err, http.StatusBadRequest, "request body too large")
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
		Expand:  optional.NewInterface([]string{routes.ExpandableFieldPayload}),
		Select_: optional.NewInterface([]string{"header.id"}),
	}
}
func optionsForBlockByStartEndHeight(startHeight, endHeight uint64) *restclient.BlocksApiBlocksGetOpts {
	return &restclient.BlocksApiBlocksGetOpts{
		Expand:      optional.NewInterface([]string{routes.ExpandableFieldPayload}),
		Select_:     optional.NewInterface([]string{"header.id", "header.height"}),
		StartHeight: optional.NewInterface(startHeight),
		EndHeight:   optional.NewInterface(endHeight),
	}
}

func optionsForBlockByHeights(heights []uint64) *restclient.BlocksApiBlocksGetOpts {
	return &restclient.BlocksApiBlocksGetOpts{
		Expand:  optional.NewInterface([]string{routes.ExpandableFieldPayload}),
		Select_: optional.NewInterface([]string{"header.id", "header.height"}),
		Height:  optional.NewInterface(heights),
	}
}

func optionsForFinalizedBlock(finalOrSealed string) *restclient.BlocksApiBlocksGetOpts {
	return &restclient.BlocksApiBlocksGetOpts{
		Expand:  optional.NewInterface([]string{routes.ExpandableFieldPayload}),
		Select_: optional.NewInterface([]string{"header.id", "header.height"}),
		Height:  optional.NewInterface(finalOrSealed),
	}
}
