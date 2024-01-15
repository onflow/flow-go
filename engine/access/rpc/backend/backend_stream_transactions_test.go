package backend


import (
	"context"
	"fmt"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/rs/zerolog"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

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

type BackendStreamTransactionsSuite struct {
	suite.Suite

	state    *protocol.State
	snapshot *protocol.Snapshot
	log      zerolog.Logger

	blocks             []*flow.Block
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

func TestBackendStreamTransactionsSuite(t *testing.T) {
	suite.Run(t, new(BackendStreamTransactionsSuite))
}

func (s *BackendStreamTransactionsSuite) SetupTest() {
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
	s.blocks = make([]*flow.Block, 0, blockCount)
	s.blockMap = make(map[uint64]*flow.Block, blockCount)
	s.blocksArray = make([]*flow.Block, 0, blockCount)
	s.sealMap = make(map[flow.Identifier]*flow.Seal, blockCount)

	// generate blockCount consecutive blocks with associated seal, result and execution data
	s.rootBlock = unittest.BlockFixture()
	parent := s.rootBlock.Header
	s.blockMap[s.rootBlock.Header.Height] = &s.rootBlock

	s.T().Logf("Generating %d blocks, root block: %d %s", blockCount, s.rootBlock.Header.Height, s.rootBlock.ID())

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

	s.snapshot.On("Head").Return(s.rootBlock.Header, nil).Maybe()
	s.state.On("Final").Return(s.snapshot, nil).Maybe()
	s.state.On("Sealed").Return(s.snapshot, nil).Maybe()

	var err error
	s.backend, err = New(s.backendParams())
	require.NoError(s.T(), err)
}

func (s *BackendStreamTransactionsSuite) TestSubscribeTransactions() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var err error

	type testType struct {
		name            string
		highestBackfill int
		startBlockID    flow.Identifier
		startHeight     uint64
		filters         state_stream.EventFilter
	}

	baseTests := []testType{
		{
			name:            "happy path - all new blocks",
			highestBackfill: -1, // no backfill
			startBlockID:    flow.ZeroID,
			startHeight:     0,
		},
		{
			name:            "happy path - partial backfill",
			highestBackfill: 2, // backfill the first 3 blocks
			startBlockID:    flow.ZeroID,
			startHeight:     s.blocks[0].Header.Height,
		},
		{
			name:            "happy path - complete backfill",
			highestBackfill: len(s.blocks) - 1, // backfill all blocks
			startBlockID:    s.blocks[0].ID(),
			startHeight:     0,
		},
		{
			name:            "happy path - start from root block by height",
			highestBackfill: len(s.blocks) - 1, // backfill all blocks
			startBlockID:    flow.ZeroID,
			startHeight:     s.backend.BlocksWatcher.RootHeight, // start from root block
		},
		{
			name:            "happy path - start from root block by id",
			highestBackfill: len(s.blocks) - 1,                   // backfill all blocks
			startBlockID:    s.backend.BlocksWatcher.RootBlockID, // start from root block
			startHeight:     0,
		},
	}

	// supports simple address comparisions for testing
	chain := flow.MonotonicEmulator.Chain()

	s.Run("Test", func() {
		s.T().Logf("len(s.execDataMap) %d", len(s.execDataMap))

		// add "backfill" block - blocks that are already in the database before the test starts
		// this simulates a subscription on a past block
		for i := 0; i <= test.highestBackfill; i++ {
			s.T().Logf("backfilling block %d", i)
			s.backend.setHighestHeight(s.blocks[i].Header.Height)
		}

		subCtx, subCancel := context.WithCancel(ctx)
		sub := s.backend.SubscribeEvents(subCtx, test.startBlockID, test.startHeight, test.filters)

		// loop over all of the blocks
		for i, b := range s.blocks {
			s.T().Logf("checking block %d %v", i, b.ID())

			// simulate new exec data received.
			// exec data for all blocks with index <= highestBackfill were already received
			if i > test.highestBackfill {
				s.backend.setHighestHeight(b.Header.Height)
				s.broadcaster.Publish()
			}

			expectedEvents := flow.EventsList{}
			for _, event := range s.blockEvents[b.ID()] {
				if test.filters.Match(event) {
					expectedEvents = append(expectedEvents, event)
				}
			}

			// consume execution data from subscription
			unittest.RequireReturnsBefore(s.T(), func() {
				v, ok := <-sub.Channel()
				require.True(s.T(), ok, "channel closed while waiting for exec data for block %d %v: err: %v", b.Header.Height, b.ID(), sub.Err())

				resp, ok := v.(*EventsResponse)
				require.True(s.T(), ok, "unexpected response type: %T", v)

				assert.Equal(s.T(), b.Header.ID(), resp.BlockID)
				assert.Equal(s.T(), b.Header.Height, resp.Height)
				assert.Equal(s.T(), expectedEvents, resp.Events)
			}, time.Second, fmt.Sprintf("timed out waiting for exec data for block %d %v", b.Header.Height, b.ID()))
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

func (s *BackendStreamTransactionsSuite) backendParams() Params {
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
