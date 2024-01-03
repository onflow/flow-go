package backend

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine"
	access "github.com/onflow/flow-go/engine/access/mock"
	backendmock "github.com/onflow/flow-go/engine/access/rpc/backend/mock"
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type BackendBlocksSuite struct {
	suite.Suite

	state    *protocol.State
	snapshot *protocol.Snapshot
	log      zerolog.Logger

	blocks             *storagemock.Blocks
	headers            *storagemock.Headers
	collections        *storagemock.Collections
	transactions       *storagemock.Transactions
	receipts           *storagemock.ExecutionReceipts
	results            *storagemock.ExecutionResults
	transactionResults *storagemock.LightTransactionResults
	seals              *storagemock.Seals

	colClient              *access.AccessAPIClient
	execClient             *access.ExecutionAPIClient
	historicalAccessClient *access.AccessAPIClient
	archiveClient          *access.AccessAPIClient

	connectionFactory *connectionmock.ConnectionFactory
	communicator      *backendmock.Communicator

	chainID flow.ChainID

	broadcaster *engine.Broadcaster
	blocksArray []*flow.Block
	blockMap    map[uint64]*flow.Block
	rootBlock   flow.Block
	sealMap     map[flow.Identifier]*flow.Seal

	backend *Backend
}

func TestBackendBlocksSuite(t *testing.T) {
	suite.Run(t, new(BackendBlocksSuite))
}

func (s *BackendBlocksSuite) SetupTest() {
	s.log = zerolog.New(zerolog.NewConsoleWriter())
	s.state = new(protocol.State)
	s.snapshot = new(protocol.Snapshot)
	header := unittest.BlockHeaderFixture()

	params := new(protocol.Params)
	params.On("FinalizedRoot").Return(header, nil)
	params.On("SporkID").Return(unittest.IdentifierFixture(), nil)
	params.On("ProtocolVersion").Return(uint(unittest.Uint64InRange(10, 30)), nil)
	params.On("SporkRootBlockHeight").Return(header.Height, nil)
	params.On("SealedRoot").Return(header, nil)
	s.state.On("Params").Return(params)

	s.blocks = new(storagemock.Blocks)
	s.headers = new(storagemock.Headers)
	s.transactions = new(storagemock.Transactions)
	s.collections = new(storagemock.Collections)
	s.receipts = new(storagemock.ExecutionReceipts)
	s.results = new(storagemock.ExecutionResults)
	s.seals = new(storagemock.Seals)
	s.colClient = new(access.AccessAPIClient)
	s.archiveClient = new(access.AccessAPIClient)
	s.execClient = new(access.ExecutionAPIClient)
	s.transactionResults = storagemock.NewLightTransactionResults(s.T())
	s.chainID = flow.Testnet
	s.historicalAccessClient = new(access.AccessAPIClient)
	s.connectionFactory = connectionmock.NewConnectionFactory(s.T())

	s.communicator = new(backendmock.Communicator)

	s.broadcaster = engine.NewBroadcaster()

	blockCount := 5
	s.blockMap = make(map[uint64]*flow.Block, blockCount)
	s.blocksArray = make([]*flow.Block, 0, blockCount)
	s.sealMap = make(map[flow.Identifier]*flow.Seal, blockCount)

	// generate blockCount consecutive blocks with associated seal, result and execution data
	s.rootBlock = unittest.BlockFixture()
	parent := s.rootBlock.Header
	s.blockMap[s.rootBlock.Header.Height] = &s.rootBlock

	for i := 0; i < blockCount; i++ {
		block := unittest.BlockWithParentFixture(parent)
		// update for next iteration
		parent = block.Header

		s.blocksArray = append(s.blocksArray, block)
		s.blockMap[block.Header.Height] = block
		seal := unittest.BlockSealsFixture(1)[0]
		s.sealMap[block.ID()] = seal
	}

	s.seals.On("FinalizedSealForBlock", mock.AnythingOfType("flow.Identifier")).Return(
		func(blockID flow.Identifier) *flow.Seal {
			if seal, ok := s.sealMap[blockID]; ok {
				return seal
			}
			return nil
		},
		func(blockID flow.Identifier) error {
			if _, ok := s.sealMap[blockID]; ok {
				return nil
			}
			return storage.ErrNotFound
		},
	).Maybe()

	s.headers.On("ByBlockID", mock.AnythingOfType("flow.Identifier")).Return(
		func(blockID flow.Identifier) *flow.Header {
			for _, block := range s.blockMap {
				if block.ID() == blockID {
					return block.Header
				}
			}
			return nil
		},
		func(blockID flow.Identifier) error {
			for _, block := range s.blockMap {
				if block.ID() == blockID {
					return nil
				}
			}
			return storage.ErrNotFound
		},
	).Maybe()

	s.headers.On("ByHeight", mock.AnythingOfType("uint64")).Return(
		func(height uint64) *flow.Header {
			if block, ok := s.blockMap[height]; ok {
				return block.Header
			}
			return nil
		},
		func(height uint64) error {
			if _, ok := s.blockMap[height]; ok {
				return nil
			}
			return storage.ErrNotFound
		},
	).Maybe()

	s.headers.On("BlockIDByHeight", mock.AnythingOfType("uint64")).Return(
		func(height uint64) flow.Identifier {
			if block, ok := s.blockMap[height]; ok {
				return block.Header.ID()
			}
			return flow.ZeroID
		},
		func(height uint64) error {
			if _, ok := s.blockMap[height]; ok {
				return nil
			}
			return storage.ErrNotFound
		},
	).Maybe()

	s.blocks.On("ByHeight", mock.AnythingOfType("uint64")).Return(
		func(height uint64) *flow.Block {
			if block, ok := s.blockMap[height]; ok {
				return block
			}
			return &flow.Block{}
		},
		func(height uint64) error {
			if _, ok := s.blockMap[height]; ok {
				return nil
			}
			return storage.ErrNotFound
		},
	).Maybe()

	s.snapshot.On("Head").Return(s.rootBlock.Header, nil).Once()
	s.state.On("Final").Return(s.snapshot, nil).Maybe()
	s.state.On("Sealed").Return(s.snapshot, nil).Maybe()

	var err error
	s.backend, err = New(s.backendParams())
	require.NoError(s.T(), err)
}

func (s *BackendBlocksSuite) backendParams() Params {
	return Params{
		State:                    s.state,
		Blocks:                   s.blocks,
		Headers:                  s.headers,
		Collections:              s.collections,
		Transactions:             s.transactions,
		ExecutionReceipts:        s.receipts,
		ExecutionResults:         s.results,
		LightTransactionResults:  s.transactionResults,
		ChainID:                  s.chainID,
		CollectionRPC:            s.colClient,
		MaxHeightRange:           DefaultMaxHeightRange,
		SnapshotHistoryLimit:     DefaultSnapshotHistoryLimit,
		Communicator:             NewNodeCommunicator(false),
		AccessMetrics:            metrics.NewNoopCollector(),
		Log:                      s.log,
		TxErrorMessagesCacheSize: 1000,
		SubscriptionParams: SubscriptionParams{
			SendTimeout:            subscription.DefaultSendTimeout,
			SendBufferSize:         subscription.DefaultSendBufferSize,
			ResponseLimit:          subscription.DefaultResponseLimit,
			Broadcaster:            s.broadcaster,
			RootHeight:             s.rootBlock.Header.Height,
			HighestAvailableHeight: s.rootBlock.Header.Height,
			Seals:                  s.seals,
		},
	}
}

func (s *BackendBlocksSuite) TestSubscribeBlocks() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type testType struct {
		name              string
		highestBackfill   int
		startBlockID      flow.Identifier
		startHeight       uint64
		blockStatus       flow.BlockStatus
		fullBlockResponse bool
	}

	baseTests := []testType{
		{
			name:            "happy path - all new blocks",
			highestBackfill: -1, // no backfill
			startBlockID:    flow.ZeroID,
			startHeight:     s.rootBlock.Header.Height,
		},
		{
			name:            "happy path - partial backfill",
			highestBackfill: 2, // backfill the first 3 blocks
			startBlockID:    flow.ZeroID,
			startHeight:     s.rootBlock.Header.Height,
		},
		{
			name:            "happy path - complete backfill",
			highestBackfill: len(s.blocksArray) - 1, // backfill all blocks
			startBlockID:    s.blocksArray[0].ID(),
			startHeight:     0,
		},
		{
			name:            "happy path - start from root block by height",
			highestBackfill: len(s.blocksArray) - 1, // backfill all blocks
			startBlockID:    flow.ZeroID,
			startHeight:     s.rootBlock.Header.Height, // start from root block
		},
		{
			name:            "happy path - start from root block by id",
			highestBackfill: len(s.blocksArray) - 1, // backfill all blocks
			startBlockID:    s.rootBlock.ID(),       // start from root block
			startHeight:     0,
		},
	}

	// create variations for each of the base test
	tests := make([]testType, 0, len(baseTests)*2)
	for _, test := range baseTests {
		t1 := test
		t1.name = fmt.Sprintf("%s - finalized blocks", test.name)
		t1.blockStatus = flow.BlockStatusFinalized
		tests = append(tests, t1)

		t2 := test
		t2.name = fmt.Sprintf("%s - sealed blocks", test.name)
		t2.blockStatus = flow.BlockStatusSealed
		tests = append(tests, t2)
	}

	for _, test := range tests {
		s.Run(test.name, func() {
			// add "backfill" block - blocks that are already in the database before the test starts
			// this simulates a subscription on a past block
			if test.highestBackfill > 0 {
				if test.blockStatus == flow.BlockStatusFinalized {
					s.backend.SetFinalizedHighestHeight(s.blocksArray[test.highestBackfill].Header.Height)
				} else {
					s.snapshot.On("Head").Return(s.blocksArray[test.highestBackfill].Header, nil)
					s.backend.SetSealedHighestHeight(s.blocksArray[test.highestBackfill].Header.Height)
				}
			}

			subCtx, subCancel := context.WithCancel(ctx)
			sub := s.backend.SubscribeBlocks(subCtx, test.startBlockID, test.startHeight, test.blockStatus)

			//loop over all blocks
			for i, b := range s.blocksArray {
				s.T().Logf("checking block %d %v %d", i, b.ID(), b.Header.Height)

				// simulate new block received.
				// all blocks with index <= highestBackfill were already received
				if int(i) > test.highestBackfill {
					if test.blockStatus == flow.BlockStatusFinalized {
						s.backend.SetFinalizedHighestHeight(b.Header.Height)
					} else {
						s.backend.SetSealedHighestHeight(b.Header.Height)
						s.snapshot.On("Head").Return(b.Header, nil)
					}
					s.broadcaster.Publish()
				}

				// consume block from subscription
				unittest.RequireReturnsBefore(s.T(), func() {
					v, ok := <-sub.Channel()
					require.True(s.T(), ok, "channel closed while waiting for exec data for block %d %v: err: %v", b.Header.Height, b.ID(), sub.Err())

					actualBlock, ok := v.(*flow.Block)
					require.True(s.T(), ok, "unexpected response type: %T", v)

					s.Require().Equal(b.Header.Height, actualBlock.Header.Height)
					s.Require().Equal(b.Header.ID(), actualBlock.Header.ID())
					s.Require().Equal(*b, *actualBlock)

				}, time.Second, fmt.Sprintf("timed out waiting for block %d %v", b.Header.Height, b.ID()))
			}

			// make sure there are no new messages waiting. the channel should be opened with nothing waiting
			unittest.RequireNeverReturnBefore(s.T(), func() {
				<-sub.Channel()
			}, 100*time.Millisecond, "timed out waiting for subscription to shutdown")

			// stop the subscription
			subCancel()

			// ensure subscription shuts down gracefully
			unittest.RequireReturnsBefore(s.T(), func() {
				v, ok := <-sub.Channel()
				assert.Nil(s.T(), v)
				assert.False(s.T(), ok)
				assert.ErrorIs(s.T(), sub.Err(), context.Canceled)
			}, 100*time.Millisecond, "timed out waiting for subscription to shutdown")
		})
	}
}

func (s *BackendBlocksSuite) TestSubscribeBlocksHandlesErrors() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Run("returns error for unknown block status", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeBlocks(subCtx, s.backend.BlocksWatcher.RootBlockID, 0, flow.BlockStatusUnknown)
		assert.Equal(s.T(), codes.InvalidArgument, status.Code(sub.Err()), "expected InvalidArgument, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})

	s.Run("returns error if both start blockID and start height are provided", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeBlocks(subCtx, unittest.IdentifierFixture(), 1, flow.BlockStatusFinalized)
		assert.Equal(s.T(), codes.InvalidArgument, status.Code(sub.Err()))
	})

	s.Run("returns error for start height before root height", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeBlocks(subCtx, flow.ZeroID, s.backend.BlocksWatcher.RootHeight-1, flow.BlockStatusFinalized)
		assert.Equal(s.T(), codes.InvalidArgument, status.Code(sub.Err()), "expected InvalidArgument, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})

	s.Run("returns error for unindexed start blockID", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeBlocks(subCtx, unittest.IdentifierFixture(), 0, flow.BlockStatusFinalized)
		assert.Equal(s.T(), codes.NotFound, status.Code(sub.Err()), "expected NotFound, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})

	s.Run("returns error for unindexed start height", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeBlocks(subCtx, flow.ZeroID, s.blocksArray[len(s.blocksArray)-1].Header.Height+10, flow.BlockStatusFinalized)
		assert.Equal(s.T(), codes.NotFound, status.Code(sub.Err()), "expected NotFound, got %v: %v", status.Code(sub.Err()).String(), sub.Err())
	})
}
