package protocol

import (
	"context"
	"crypto/md5"
	"math/rand"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"

	access "github.com/onflow/flow-go/engine/access/mock"
	access_backend "github.com/onflow/flow-go/engine/access/rpc/backend"
	backendmock "github.com/onflow/flow-go/engine/access/rpc/backend/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
)

// type Suite struct {
// 	suite.Suite

// 	state    *protocol.State
// 	snapshot *protocol.Snapshot
// 	log      zerolog.Logger

// 	blocks                 *storagemock.Blocks
// 	headers                *storagemock.Headers
// 	collections            *storagemock.Collections
// 	transactions           *storagemock.Transactions
// 	receipts               *storagemock.ExecutionReceipts
// 	results                *storagemock.ExecutionResults
// 	colClient              *access.AccessAPIClient
// 	execClient             *access.ExecutionAPIClient
// 	historicalAccessClient *access.AccessAPIClient
// 	connectionFactory      *backendmock.ConnectionFactory
// 	chainID                flow.ChainID
// 	//metrics    			   *metrics.NoopCollector
// }

type Suite struct {
	suite.Suite

	state    *protocol.State
	snapshot *protocol.Snapshot
	log      zerolog.Logger

	blocks                 *storagemock.Blocks
	headers                *storagemock.Headers
	collections            *storagemock.Collections
	transactions           *storagemock.Transactions
	receipts               *storagemock.ExecutionReceipts
	results                *storagemock.ExecutionResults
	colClient              *access.AccessAPIClient
	execClient             *access.ExecutionAPIClient
	historicalAccessClient *access.AccessAPIClient
	connectionFactory      *backendmock.ConnectionFactory
	chainID                flow.ChainID
}

func TestHandler(t *testing.T) {
	suite.Run(t, new(Suite))
}

// func (suite *Suite) SetupTest() {
// 	rand.Seed(time.Now().UnixNano())
// 	suite.log = zerolog.New(zerolog.NewConsoleWriter())
// 	suite.state = new(protocol.State)
// 	suite.snapshot = new(protocol.Snapshot)
// 	header := unittest.BlockHeaderFixture()
// 	params := new(protocol.Params)
// 	params.On("Root").Return(&header, nil)
// 	suite.state.On("Params").Return(params).Maybe()
// 	suite.blocks = new(storagemock.Blocks)
// 	suite.headers = new(storagemock.Headers)
// 	suite.transactions = new(storagemock.Transactions)
// 	suite.collections = new(storagemock.Collections)
// 	suite.receipts = new(storagemock.ExecutionReceipts)
// 	suite.results = new(storagemock.ExecutionResults)
// 	suite.colClient = new(access.AccessAPIClient)
// 	suite.execClient = new(access.ExecutionAPIClient)
// 	suite.chainID = flow.Testnet
// 	suite.historicalAccessClient = new(access.AccessAPIClient)
// 	suite.connectionFactory = new(backendmock.ConnectionFactory)
// }

func (suite *Suite) SetupTest() {
	rand.Seed(time.Now().UnixNano())
	suite.log = zerolog.New(zerolog.NewConsoleWriter())
	suite.state = new(protocol.State)
	suite.snapshot = new(protocol.Snapshot)
	header := unittest.BlockHeaderFixture()
	params := new(protocol.Params)
	params.On("Root").Return(&header, nil)
	suite.state.On("Params").Return(params).Maybe()
	suite.blocks = new(storagemock.Blocks)
	suite.headers = new(storagemock.Headers)
	suite.transactions = new(storagemock.Transactions)
	suite.collections = new(storagemock.Collections)
	suite.receipts = new(storagemock.ExecutionReceipts)
	suite.results = new(storagemock.ExecutionResults)
	suite.colClient = new(access.AccessAPIClient)
	suite.execClient = new(access.ExecutionAPIClient)
	suite.chainID = flow.Testnet
	suite.historicalAccessClient = new(access.AccessAPIClient)
	suite.connectionFactory = new(backendmock.ConnectionFactory)
}

func (suite *Suite) TestGetLatestFinalizedBlockHeader() {
	// setup the mocks
	block := unittest.BlockHeaderFixture()
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Head").Return(&block, nil).Once()

	backend := NewBackend(
		suite.state,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		suite.chainID,
		metrics.NewNoopCollector(),
		nil,
		false,
		access_backend.DefaultMaxHeightRange,
		nil,
		nil,
		suite.log,
		access_backend.DefaultSnapshotHistoryLimit,
	)
	// query the handler for the latest finalized block
	header, err := backend.GetLatestBlockHeader(context.Background(), false)
	suite.checkResponse(header, err)

	// make sure we got the latest block
	suite.Require().Equal(block.ID(), header.ID())
	suite.Require().Equal(block.Height, header.Height)
	suite.Require().Equal(block.ParentID, header.ParentID)

	suite.assertAllExpectations()

}

func (suite *Suite) checkResponse(resp interface{}, err error) {
	suite.Require().NoError(err)
	suite.Require().NotNil(resp)
}

func (suite *Suite) assertAllExpectations() {
	suite.snapshot.AssertExpectations(suite.T())
	suite.state.AssertExpectations(suite.T())
	suite.blocks.AssertExpectations(suite.T())
	suite.headers.AssertExpectations(suite.T())
	suite.collections.AssertExpectations(suite.T())
	suite.transactions.AssertExpectations(suite.T())
	suite.execClient.AssertExpectations(suite.T())
}

func NewBackend(
	state protocol.State,
	collectionRPC accessproto.AccessAPIClient,
	historicalAccessNodes []accessproto.AccessAPIClient,
	blocks storage.Blocks,
	headers storage.Headers,
	collections storage.Collections,
	transactions storage.Transactions,
	executionReceipts storage.ExecutionReceipts,
	executionResults storage.ExecutionResults,
	chainID flow.ChainID,
	transactionMetrics module.TransactionMetrics,
	connFactory access_backend.ConnectionFactory,
	retryEnabled bool,
	maxHeightRange uint,
	preferredExecutionNodeIDs []string,
	fixedExecutionNodeIDs []string,
	log zerolog.Logger,
	snapshotHistoryLimit int,
) *access_backend.Backend {
	retry := access_backend.newRetry()
	if retryEnabled {
		retry.Activate()
	}

	b := &access_backend.Backend{
		state: state,
		// create the sub-backends
		backendScripts: backendScripts{
			headers:           headers,
			executionReceipts: executionReceipts,
			connFactory:       connFactory,
			state:             state,
			log:               log,
			seenScripts:       make(map[[md5.Size]byte]time.Time),
		},
		backendTransactions: backendTransactions{
			staticCollectionRPC:  collectionRPC,
			state:                state,
			chainID:              chainID,
			collections:          collections,
			blocks:               blocks,
			transactions:         transactions,
			executionReceipts:    executionReceipts,
			transactionValidator: configureTransactionValidator(state, chainID),
			transactionMetrics:   transactionMetrics,
			retry:                retry,
			connFactory:          connFactory,
			previousAccessNodes:  historicalAccessNodes,
			log:                  log,
		},
		backendEvents: backendEvents{
			state:             state,
			headers:           headers,
			executionReceipts: executionReceipts,
			connFactory:       connFactory,
			log:               log,
			maxHeightRange:    maxHeightRange,
		},
		backendBlockHeaders: backendBlockHeaders{
			headers: headers,
			state:   state,
		},
		backendBlockDetails: backendBlockDetails{
			blocks: blocks,
			state:  state,
		},
		backendAccounts: backendAccounts{
			state:             state,
			headers:           headers,
			executionReceipts: executionReceipts,
			connFactory:       connFactory,
			log:               log,
		},
		backendExecutionResults: backendExecutionResults{
			executionResults: executionResults,
		},
		collections:          collections,
		executionReceipts:    executionReceipts,
		connFactory:          connFactory,
		chainID:              chainID,
		snapshotHistoryLimit: snapshotHistoryLimit,
	}

	retry.SetBackend(b)

	var err error
	preferredENIdentifiers, err = identifierList(preferredExecutionNodeIDs)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to convert node id string to Flow Identifier for preferred EN map")
	}

	fixedENIdentifiers, err = identifierList(fixedExecutionNodeIDs)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to convert node id string to Flow Identifier for fixed EN map")
	}

	return b
}

// func (suite *Suite) RunTest(
// 	f func(handler *protocol.ProtocolAPI, db *badger.DB, blocks *storage.Blocks, headers *storage.Headers, results *storage.ExecutionResults),
// ) {
// 	unittest.RunWithBadgerDB(suite.T(), func(db *badger.DB) {
// 		headers, _, _, _, _, blocks, _, _, _, results := util.StorageLayer(suite.T(), db)
// 		transactions := storage.NewTransactions(suite.metrics, db)
// 		collections := storage.NewCollections(db, transactions)
// 		receipts := storage.NewExecutionReceipts(suite.metrics, db, results, storage.DefaultCacheSize)

// 		suite.backend = backend.New(suite.state,
// 			suite.collClient,
// 			nil,
// 			blocks,
// 			headers,
// 			collections,
// 			transactions,
// 			receipts,
// 			results,
// 			suite.chainID,
// 			suite.metrics,
// 			nil,
// 			false,
// 			backend.DefaultMaxHeightRange,
// 			nil,
// 			nil,
// 			suite.log,
// 			backend.DefaultSnapshotHistoryLimit,
// 		)

// 		handler := access.NewHandler(suite.backend, suite.chainID.Chain())

// 		f(handler, db, blocks, headers, results)
// 	})
// }

// func (suite *Suite) TestGetBlockByIDAndHeight() {
// 	suite.RunTest(func(handler *access.Handler, db *badger.DB, blocks *storage.Blocks, _ *storage.Headers, _ *storage.ExecutionResults) {

// 		// test block1 get by ID
// 		block1 := unittest.BlockFixture()
// 		// test block2 get by height
// 		block2 := unittest.BlockFixture()
// 		block2.Header.Height = 2

// 		require.NoError(suite.T(), blocks.Store(&block1))
// 		require.NoError(suite.T(), blocks.Store(&block2))

// 		// the follower logic should update height index on the block storage when a block is finalized
// 		err := db.Update(operation.IndexBlockHeight(block2.Header.Height, block2.ID()))
// 		require.NoError(suite.T(), err)

// 		assertHeaderResp := func(resp *accessproto.BlockHeaderResponse, err error, header *flow.Header) {
// 			require.NoError(suite.T(), err)
// 			require.NotNil(suite.T(), resp)
// 			actual := *resp.Block
// 			expectedMessage, err := convert.BlockHeaderToMessage(header)
// 			require.NoError(suite.T(), err)
// 			require.Equal(suite.T(), *expectedMessage, actual)
// 			expectedBlockHeader, err := convert.MessageToBlockHeader(&actual)
// 			require.NoError(suite.T(), err)
// 			require.Equal(suite.T(), expectedBlockHeader, header)
// 		}

// 		assertBlockResp := func(resp *accessproto.BlockResponse, err error, block *flow.Block) {
// 			require.NoError(suite.T(), err)
// 			require.NotNil(suite.T(), resp)
// 			actual := resp.Block
// 			expectedMessage, err := convert.BlockToMessage(block)
// 			require.NoError(suite.T(), err)
// 			require.Equal(suite.T(), expectedMessage, actual)
// 			expectedBlock, err := convert.MessageToBlock(resp.Block)
// 			require.NoError(suite.T(), err)
// 			require.Equal(suite.T(), expectedBlock.ID(), block.ID())
// 		}

// 		assertLightBlockResp := func(resp *accessproto.BlockResponse, err error, block *flow.Block) {
// 			require.NoError(suite.T(), err)
// 			require.NotNil(suite.T(), resp)
// 			actual := resp.Block
// 			expectedMessage := convert.BlockToMessageLight(block)
// 			require.Equal(suite.T(), expectedMessage, actual)
// 		}

// 		suite.Run("get header 1 by ID", func() {
// 			// get header by ID
// 			id := block1.ID()
// 			req := &accessproto.GetBlockHeaderByIDRequest{
// 				Id: id[:],
// 			}

// 			resp, err := handler.GetBlockHeaderByID(context.Background(), req)

// 			// assert it is indeed block1
// 			assertHeaderResp(resp, err, block1.Header)
// 		})

// 		suite.Run("get block 1 by ID", func() {
// 			id := block1.ID()
// 			// get block details by ID
// 			req := &accessproto.GetBlockByIDRequest{
// 				Id:                id[:],
// 				FullBlockResponse: true,
// 			}

// 			resp, err := handler.GetBlockByID(context.Background(), req)

// 			assertBlockResp(resp, err, &block1)
// 		})

// 		suite.Run("get block light 1 by ID", func() {
// 			id := block1.ID()
// 			// get block details by ID
// 			req := &accessproto.GetBlockByIDRequest{
// 				Id: id[:],
// 			}

// 			resp, err := handler.GetBlockByID(context.Background(), req)

// 			assertLightBlockResp(resp, err, &block1)
// 		})

// 		suite.Run("get header 2 by height", func() {

// 			// get header by height
// 			req := &accessproto.GetBlockHeaderByHeightRequest{
// 				Height: block2.Header.Height,
// 			}

// 			resp, err := handler.GetBlockHeaderByHeight(context.Background(), req)

// 			assertHeaderResp(resp, err, block2.Header)
// 		})

// 		suite.Run("get block 2 by height", func() {
// 			// get block details by height
// 			req := &accessproto.GetBlockByHeightRequest{
// 				Height:            block2.Header.Height,
// 				FullBlockResponse: true,
// 			}

// 			resp, err := handler.GetBlockByHeight(context.Background(), req)

// 			assertBlockResp(resp, err, &block2)
// 		})

// 		suite.Run("get block 2 by height", func() {
// 			// get block details by height
// 			req := &accessproto.GetBlockByHeightRequest{
// 				Height: block2.Header.Height,
// 			}

// 			resp, err := handler.GetBlockByHeight(context.Background(), req)
// 			assertLightBlockResp(resp, err, &block2)
// 		})
// 	})
// }
