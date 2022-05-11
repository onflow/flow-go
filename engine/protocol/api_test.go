package protocol

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	entitiesproto "github.com/onflow/flow/protobuf/go/flow/entities"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	bprotocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/util"

	access "github.com/onflow/flow-go/engine/access/mock"
	backendmock "github.com/onflow/flow-go/engine/access/rpc/backend/mock"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

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
	metrics    			   *metrics.NoopCollector
	historicalAccessClient *access.AccessAPIClient
	connectionFactory      *backendmock.ConnectionFactory
	chainID                flow.ChainID
}

func (suite *Suite) TestGetLatestFinalizedBlockHeader() {
	// setup the mocks
	block := unittest.BlockHeaderFixture()
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Head").Return(&block, nil).Once()

	backend := New(
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
		DefaultMaxHeightRange,
		nil,
		nil,
		suite.log,
		DefaultSnapshotHistoryLimit,
	)

	// query the handler for the latest finalized block
	header, err := protocol.GetLatestBlockHeader(context.Background(), false)
	suite.checkResponse(header, err)

	// make sure we got the latest block
	suite.Require().Equal(block.ID(), header.ID())
	suite.Require().Equal(block.Height, header.Height)
	suite.Require().Equal(block.ParentID, header.ParentID)

	suite.assertAllExpectations()

}

func (suite *Suite) RunTest(
	f func(handler *protocol.API, db *badger.DB, blocks *storage.Blocks, headers *storage.Headers, results *storage.ExecutionResults),
) {
	unittest.RunWithBadgerDB(suite.T(), func(db *badger.DB) {
		headers, _, _, _, _, blocks, _, _, _, results := util.StorageLayer(suite.T(), db)
		transactions := storage.NewTransactions(suite.metrics, db)
		collections := storage.NewCollections(db, transactions)
		receipts := storage.NewExecutionReceipts(suite.metrics, db, results, storage.DefaultCacheSize)

		suite.backend = backend.New(suite.state,
			suite.collClient,
			nil,
			blocks,
			headers,
			collections,
			transactions,
			receipts,
			results,
			suite.chainID,
			suite.metrics,
			nil,
			false,
			backend.DefaultMaxHeightRange,
			nil,
			nil,
			suite.log,
			backend.DefaultSnapshotHistoryLimit,
		)

		handler := access.NewHandler(suite.backend, suite.chainID.Chain())

		f(handler, db, blocks, headers, results)
	})
}

func (suite *Suite) TestGetBlockByIDAndHeight() {
	suite.RunTest(func(handler *access.Handler, db *badger.DB, blocks *storage.Blocks, _ *storage.Headers, _ *storage.ExecutionResults) {

		// test block1 get by ID
		block1 := unittest.BlockFixture()
		// test block2 get by height
		block2 := unittest.BlockFixture()
		block2.Header.Height = 2

		require.NoError(suite.T(), blocks.Store(&block1))
		require.NoError(suite.T(), blocks.Store(&block2))

		// the follower logic should update height index on the block storage when a block is finalized
		err := db.Update(operation.IndexBlockHeight(block2.Header.Height, block2.ID()))
		require.NoError(suite.T(), err)

		assertHeaderResp := func(resp *accessproto.BlockHeaderResponse, err error, header *flow.Header) {
			require.NoError(suite.T(), err)
			require.NotNil(suite.T(), resp)
			actual := *resp.Block
			expectedMessage, err := convert.BlockHeaderToMessage(header)
			require.NoError(suite.T(), err)
			require.Equal(suite.T(), *expectedMessage, actual)
			expectedBlockHeader, err := convert.MessageToBlockHeader(&actual)
			require.NoError(suite.T(), err)
			require.Equal(suite.T(), expectedBlockHeader, header)
		}

		assertBlockResp := func(resp *accessproto.BlockResponse, err error, block *flow.Block) {
			require.NoError(suite.T(), err)
			require.NotNil(suite.T(), resp)
			actual := resp.Block
			expectedMessage, err := convert.BlockToMessage(block)
			require.NoError(suite.T(), err)
			require.Equal(suite.T(), expectedMessage, actual)
			expectedBlock, err := convert.MessageToBlock(resp.Block)
			require.NoError(suite.T(), err)
			require.Equal(suite.T(), expectedBlock.ID(), block.ID())
		}

		assertLightBlockResp := func(resp *accessproto.BlockResponse, err error, block *flow.Block) {
			require.NoError(suite.T(), err)
			require.NotNil(suite.T(), resp)
			actual := resp.Block
			expectedMessage := convert.BlockToMessageLight(block)
			require.Equal(suite.T(), expectedMessage, actual)
		}

		suite.Run("get header 1 by ID", func() {
			// get header by ID
			id := block1.ID()
			req := &accessproto.GetBlockHeaderByIDRequest{
				Id: id[:],
			}

			resp, err := handler.GetBlockHeaderByID(context.Background(), req)

			// assert it is indeed block1
			assertHeaderResp(resp, err, block1.Header)
		})

		suite.Run("get block 1 by ID", func() {
			id := block1.ID()
			// get block details by ID
			req := &accessproto.GetBlockByIDRequest{
				Id:                id[:],
				FullBlockResponse: true,
			}

			resp, err := handler.GetBlockByID(context.Background(), req)

			assertBlockResp(resp, err, &block1)
		})

		suite.Run("get block light 1 by ID", func() {
			id := block1.ID()
			// get block details by ID
			req := &accessproto.GetBlockByIDRequest{
				Id: id[:],
			}

			resp, err := handler.GetBlockByID(context.Background(), req)

			assertLightBlockResp(resp, err, &block1)
		})

		suite.Run("get header 2 by height", func() {

			// get header by height
			req := &accessproto.GetBlockHeaderByHeightRequest{
				Height: block2.Header.Height,
			}

			resp, err := handler.GetBlockHeaderByHeight(context.Background(), req)

			assertHeaderResp(resp, err, block2.Header)
		})

		suite.Run("get block 2 by height", func() {
			// get block details by height
			req := &accessproto.GetBlockByHeightRequest{
				Height:            block2.Header.Height,
				FullBlockResponse: true,
			}

			resp, err := handler.GetBlockByHeight(context.Background(), req)

			assertBlockResp(resp, err, &block2)
		})

		suite.Run("get block 2 by height", func() {
			// get block details by height
			req := &accessproto.GetBlockByHeightRequest{
				Height: block2.Header.Height,
			}

			resp, err := handler.GetBlockByHeight(context.Background(), req)
			assertLightBlockResp(resp, err, &block2)
		})
	})
}
