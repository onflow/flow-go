package protocol

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	badger_storage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/util"
	"github.com/onflow/flow-go/utils/unittest"
	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
)

type Suite struct {
	suite.Suite

	state    *protocol.State
	snapshot *protocol.Snapshot
	log      zerolog.Logger

	blocks  *storagemock.Blocks
	headers *storagemock.Headers
}

func TestHandler(t *testing.T) {
	suite.Run(t, new(Suite))
	suite.Run(t, new(IdHeightSuite))
}

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
}

func (suite *Suite) TestGetLatestFinalizedBlockHeader() {
	// setup the mocks
	block := unittest.BlockHeaderFixture()
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Head").Return(&block, nil).Once()

	backend := New(suite.state, suite.blocks, suite.headers)

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
}

func (suite *Suite) TestGetLatestFinalizedBlock() {
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()

	// setup the mocks
	expected := unittest.BlockFixture()
	header := expected.Header

	suite.snapshot.
		On("Head").
		Return(header, nil).
		Once()

	suite.blocks.
		On("ByID", header.ID()).
		Return(&expected, nil).
		Once()

	backend := New(suite.state, suite.blocks, suite.headers)

	// query the handler for the latest finalized header
	actual, err := backend.GetLatestBlock(context.Background(), false)
	suite.checkResponse(actual, err)

	// make sure we got the latest header
	suite.Require().Equal(expected, *actual)

	suite.assertAllExpectations()
}

type IdHeightSuite struct {
	suite.Suite
	state    *protocol.State
	snapshot *protocol.Snapshot
	log      zerolog.Logger
	chainID  flow.ChainID
	metrics  *metrics.NoopCollector
	blocks   *storagemock.Blocks
	headers  *storagemock.Headers
}

func (suite *IdHeightSuite) RunTest(
	f func(db *badger.DB, blocks *badger_storage.Blocks, headers *badger_storage.Headers, results *badger_storage.ExecutionResults),
) {
	unittest.RunWithBadgerDB(suite.T(), func(db *badger.DB) {
		headers, _, _, _, _, blocks, _, _, _, results := util.StorageLayer(suite.T(), db)
		f(db, blocks, headers, results)
	})
}

func (suite *IdHeightSuite) TestGetBlockByIDAndHeight() {
	suite.RunTest(func(db *badger.DB, blocks *badger_storage.Blocks, _ *badger_storage.Headers, _ *badger_storage.ExecutionResults) {

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

			resp, err := API.GetBlockHeaderByID(context.Background(), req)

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

			resp, err := API.GetBlockByID(context.Background(), req)

			assertBlockResp(resp, err, &block1)
		})

		suite.Run("get block light 1 by ID", func() {
			id := block1.ID()
			// get block details by ID
			req := &accessproto.GetBlockByIDRequest{
				Id: id[:],
			}

			resp, err := API.GetBlockByID(context.Background(), req)

			assertLightBlockResp(resp, err, &block1)
		})

		suite.Run("get header 2 by height", func() {

			// get header by height
			req := &accessproto.GetBlockHeaderByHeightRequest{
				Height: block2.Header.Height,
			}

			resp, err := API.GetBlockHeaderByHeight(context.Background(), req)

			assertHeaderResp(resp, err, block2.Header)
		})

		suite.Run("get block 2 by height", func() {
			// get block details by height
			req := &accessproto.GetBlockByHeightRequest{
				Height:            block2.Header.Height,
				FullBlockResponse: true,
			}

			resp, err := API.GetBlockByHeight(context.Background(), req)

			assertBlockResp(resp, err, &block2)
		})

		suite.Run("get block 2 by height", func() {
			// get block details by height
			req := &accessproto.GetBlockByHeightRequest{
				Height: block2.Header.Height,
			}

			resp, err := API.GetBlockByHeight(context.Background(), req)

			assertLightBlockResp(resp, err, &block2)
		})
	})
}