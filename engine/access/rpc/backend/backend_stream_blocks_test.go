package backend

import (
	"context"
	"fmt"
	"testing"
	"testing/synctest"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/rpc/backend/events"
	"github.com/onflow/flow-go/engine/access/rpc/backend/query_mode"
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	"github.com/onflow/flow-go/engine/access/subscription_old"
	"github.com/onflow/flow-go/engine/access/subscription_old/tracker"
	"github.com/onflow/flow-go/model/flow"
	osyncmock "github.com/onflow/flow-go/module/executiondatasync/optimistic_sync/mock"
	"github.com/onflow/flow-go/module/metrics"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

// BackendBlocksSuite is a test suite for the backendBlocks functionality related to blocks subscription.
// It utilizes the suite to organize and structure test code.
type BackendBlocksSuite struct {
	suite.Suite

	state    *protocol.State
	snapshot *protocol.Snapshot
	log      zerolog.Logger

	blocks       *storagemock.Blocks
	headers      *storagemock.Headers
	blockTracker tracker.BlockTracker

	connectionFactory *connectionmock.ConnectionFactory

	chainID flow.ChainID

	broadcaster *engine.Broadcaster
	blocksArray []*flow.Block
	blockMap    map[uint64]*flow.Block
	rootBlock   *flow.Block

	executionResultInfoProvider *osyncmock.ExecutionResultInfoProvider
	executionStateCache         *osyncmock.ExecutionStateCache
	executionDataSnapshot       *osyncmock.Snapshot

	backend *Backend
}

// testType represents a test scenario for subscribing
type testType struct {
	name            string
	highestBackfill int
	startValue      interface{}
	blockStatus     flow.BlockStatus
	expectedBlocks  []*flow.Block
}

func TestBackendBlocksSuite(t *testing.T) {
	suite.Run(t, new(BackendBlocksSuite))
}

// SetupTest initializes the test suite with required dependencies.
func (s *BackendBlocksSuite) SetupTest() {
	s.log = unittest.Logger()
	s.state = new(protocol.State)
	s.snapshot = new(protocol.Snapshot)
	header := unittest.BlockHeaderFixture()

	params := new(protocol.Params)
	params.On("SporkID").Return(unittest.IdentifierFixture(), nil)
	params.On("SporkRootBlockHeight").Return(header.Height, nil)
	params.On("SealedRoot").Return(header, nil)
	s.state.On("Params").Return(params)

	s.blocks = new(storagemock.Blocks)
	s.headers = new(storagemock.Headers)
	s.chainID = flow.Testnet
	s.connectionFactory = connectionmock.NewConnectionFactory(s.T())

	blockCount := 5
	s.blockMap = make(map[uint64]*flow.Block, blockCount)
	s.blocksArray = make([]*flow.Block, 0, blockCount)

	// generate blockCount consecutive blocks with associated seal, result and execution data
	s.rootBlock = unittest.BlockFixture()
	parent := s.rootBlock.ToHeader()
	s.blockMap[s.rootBlock.Height] = s.rootBlock

	s.T().Logf("Generating %d blocks, root block: %d %s", blockCount, s.rootBlock.Height, s.rootBlock.ID())
	for i := 0; i < blockCount; i++ {
		block := unittest.BlockWithParentFixture(parent)
		// update for next iteration
		parent = block.ToHeader()

		s.blocksArray = append(s.blocksArray, block)
		s.blockMap[block.Height] = block
		s.T().Logf("Adding block %d %s", block.Height, block.ID())
	}

	s.headers.On("ByBlockID", mock.AnythingOfType("flow.Identifier")).Return(
		func(blockID flow.Identifier) (*flow.Header, error) {
			for _, block := range s.blockMap {
				if block.ID() == blockID {
					return block.ToHeader(), nil
				}
			}
			return nil, storage.ErrNotFound
		},
	).Maybe()

	s.headers.On("ByHeight", mock.AnythingOfType("uint64")).Return(
		mocks.ConvertStorageOutput(
			mocks.StorageMapGetter(s.blockMap),
			func(block *flow.Block) *flow.Header { return block.ToHeader() },
		),
	).Maybe()

	s.blocks.On("ByHeight", mock.AnythingOfType("uint64")).Return(
		mocks.StorageMapGetter(s.blockMap),
	).Maybe()

	s.state.On("Final").Return(s.snapshot, nil).Maybe()
	s.state.On("Sealed").Return(s.snapshot, nil).Maybe()

	s.executionResultInfoProvider = osyncmock.NewExecutionResultInfoProvider(s.T())
	s.executionDataSnapshot = osyncmock.NewSnapshot(s.T())
	s.executionStateCache = osyncmock.NewExecutionStateCache(s.T())
}

// backendParams returns the Params configuration for the backend.
func (s *BackendBlocksSuite) backendParams(broadcaster *engine.Broadcaster) Params {
	var err error
	// Head() is called twice by NewBlockTracker
	s.snapshot.On("Head").Return(s.rootBlock.ToHeader(), nil).Twice()
	s.blockTracker, err = tracker.NewBlockTracker(
		s.state,
		s.rootBlock.Height,
		s.headers,
		broadcaster,
	)
	s.Require().NoError(err)

	return Params{
		State:                s.state,
		Blocks:               s.blocks,
		Headers:              s.headers,
		ChainID:              s.chainID,
		MaxHeightRange:       events.DefaultMaxHeightRange,
		SnapshotHistoryLimit: DefaultSnapshotHistoryLimit,
		AccessMetrics:        metrics.NewNoopCollector(),
		Log:                  s.log,
		SubscriptionHandler: subscription_old.NewSubscriptionHandler(
			s.log,
			broadcaster,
			subscription_old.DefaultSendTimeout,
			subscription_old.DefaultResponseLimit,
			subscription_old.DefaultSendBufferSize,
		),
		BlockTracker:                s.blockTracker,
		EventQueryMode:              query_mode.IndexQueryModeExecutionNodesOnly,
		ScriptExecutionMode:         query_mode.IndexQueryModeExecutionNodesOnly,
		TxResultQueryMode:           query_mode.IndexQueryModeExecutionNodesOnly,
		ExecutionResultInfoProvider: s.executionResultInfoProvider,
		ExecutionStateCache:         s.executionStateCache,
	}
}

// subscribeFromStartBlockIdTestCases generates variations of testType scenarios for subscriptions
// starting from a specified block ID. It is designed to test the subscription functionality when the subscription
// starts from a custom block ID, either sealed or finalized.
func (s *BackendBlocksSuite) subscribeFromStartBlockIdTestCases() []testType {
	expectedFromRoot := []*flow.Block{s.rootBlock}
	expectedFromRoot = append(expectedFromRoot, s.blocksArray...)

	baseTests := []testType{
		{
			name:            "happy path - all new blocks",
			highestBackfill: -1, // no backfill
			startValue:      s.rootBlock.ID(),
			expectedBlocks:  expectedFromRoot,
		},
		{
			name:            "happy path - partial backfill",
			highestBackfill: 2, // backfill the first 3 blocks
			startValue:      s.blocksArray[0].ID(),
			expectedBlocks:  s.blocksArray,
		},
		{
			name:            "happy path - complete backfill",
			highestBackfill: len(s.blocksArray) - 1, // backfill all blocks
			startValue:      s.blocksArray[0].ID(),
			expectedBlocks:  s.blocksArray,
		},
		{
			name:            "happy path - start from root block by id",
			highestBackfill: len(s.blocksArray) - 1, // backfill all blocks
			startValue:      s.rootBlock.ID(),       // start from root block
			expectedBlocks:  expectedFromRoot,
		},
	}

	return s.setupBlockStatusesForTestCases(baseTests)
}

// subscribeFromStartHeightTestCases generates variations of testType scenarios for subscriptions
// starting from a specified block height. It is designed to test the subscription functionality when the subscription
// starts from a custom height, either sealed or finalized.
func (s *BackendBlocksSuite) subscribeFromStartHeightTestCases() []testType {
	expectedFromRoot := []*flow.Block{s.rootBlock}
	expectedFromRoot = append(expectedFromRoot, s.blocksArray...)

	baseTests := []testType{
		{
			name:            "happy path - all new blocks",
			highestBackfill: -1, // no backfill
			startValue:      s.rootBlock.Height,
			expectedBlocks:  expectedFromRoot,
		},
		{
			name:            "happy path - partial backfill",
			highestBackfill: 2, // backfill the first 3 blocks
			startValue:      s.blocksArray[0].Height,
			expectedBlocks:  s.blocksArray,
		},
		{
			name:            "happy path - complete backfill",
			highestBackfill: len(s.blocksArray) - 1, // backfill all blocks
			startValue:      s.blocksArray[0].Height,
			expectedBlocks:  s.blocksArray,
		},
		{
			name:            "happy path - start from root block by id",
			highestBackfill: len(s.blocksArray) - 1, // backfill all blocks
			startValue:      s.rootBlock.Height,     // start from root block
			expectedBlocks:  expectedFromRoot,
		},
	}

	return s.setupBlockStatusesForTestCases(baseTests)
}

// subscribeFromLatestTestCases generates variations of testType scenarios for subscriptions
// starting from the latest sealed block. It is designed to test the subscription functionality when the subscription
// starts from the latest available block, either sealed or finalized.
func (s *BackendBlocksSuite) subscribeFromLatestTestCases() []testType {
	expectedFromRoot := []*flow.Block{s.rootBlock}
	expectedFromRoot = append(expectedFromRoot, s.blocksArray...)

	baseTests := []testType{
		{
			name:            "happy path - all new blocks",
			highestBackfill: -1, // no backfill
			expectedBlocks:  expectedFromRoot,
		},
		{
			name:            "happy path - partial backfill",
			highestBackfill: 2, // backfill the first 3 blocks
			expectedBlocks:  expectedFromRoot,
		},
		{
			name:            "happy path - complete backfill",
			highestBackfill: len(s.blocksArray) - 1, // backfill all blocks
			expectedBlocks:  expectedFromRoot,
		},
	}

	return s.setupBlockStatusesForTestCases(baseTests)
}

// setupBlockStatusesForTestCases sets up variations for each of the base test cases.
// The function performs the following actions:
//
//  1. Creates variations for each of the provided base test scenarios.
//  2. For each base test, it generates two variations: one for Sealed blocks and one for Finalized blocks.
//  3. Returns a slice of testType containing all variations of test scenarios.
//
// Parameters:
//   - baseTests: A slice of testType representing base test scenarios.
func (s *BackendBlocksSuite) setupBlockStatusesForTestCases(baseTests []testType) []testType {
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

	return tests
}

// setupBlockTrackerMock configures a mock for the block tracker based on the provided parameters.
//
// Parameters:
//   - blockStatus: The status of the blocks being tracked (Sealed or Finalized).
//   - highestHeader: The highest header that the block tracker should report.
func (s *BackendBlocksSuite) setupBlockTrackerMock(blockStatus flow.BlockStatus, highestHeader *flow.Header) {
	s.snapshot.On("Head").Unset()
	s.snapshot.On("Head").Return(highestHeader, nil)
	err := s.blockTracker.ProcessOnFinalizedBlock()
	s.Require().NoError(err)
}

// TestSubscribeBlocksFromStartBlockID tests the SubscribeBlocksFromStartBlockID method.
func (s *BackendBlocksSuite) TestSubscribeBlocksFromStartBlockID() {
	call := func(ctx context.Context, startValue interface{}, blockStatus flow.BlockStatus) subscription_old.Subscription {
		return s.backend.SubscribeBlocksFromStartBlockID(ctx, startValue.(flow.Identifier), blockStatus)
	}

	s.subscribe(call, s.requireBlocks, s.subscribeFromStartBlockIdTestCases())
}

// TestSubscribeBlocksFromStartHeight tests the SubscribeBlocksFromStartHeight method.
func (s *BackendBlocksSuite) TestSubscribeBlocksFromStartHeight() {
	call := func(ctx context.Context, startValue interface{}, blockStatus flow.BlockStatus) subscription_old.Subscription {
		return s.backend.SubscribeBlocksFromStartHeight(ctx, startValue.(uint64), blockStatus)
	}

	s.subscribe(call, s.requireBlocks, s.subscribeFromStartHeightTestCases())
}

// TestSubscribeBlocksFromLatest tests the SubscribeBlocksFromLatest method.
func (s *BackendBlocksSuite) TestSubscribeBlocksFromLatest() {
	call := func(ctx context.Context, startValue interface{}, blockStatus flow.BlockStatus) subscription_old.Subscription {
		return s.backend.SubscribeBlocksFromLatest(ctx, blockStatus)
	}

	s.subscribe(call, s.requireBlocks, s.subscribeFromLatestTestCases())
}

// subscribe is the common method with tests the functionality of the subscribe methods in the Backend.
// It covers various scenarios for subscribing, handling backfill, and receiving block updates.
// The test cases include scenarios for both finalized and sealed blocks.
//
// Parameters:
//
//   - subscribeFn: A function representing the subscription method to be tested.
//     It takes a context, startValue, and blockStatus as parameters
//     and returns a subscription.Subscription.
//
//   - requireFn: A function responsible for validating that the received information
//     matches the expected data. It takes an actual interface{} and an expected *flow.Block as parameters.
//
//   - tests: A slice of testType representing different test scenarios for subscriptions.
//
// The function performs the following steps for each test case:
//
//  1. Initializes the test context and cancellation function.
//  2. Iterates through the provided test cases.
//  3. For each test case, sets up a block tracker mock if there are blocks to backfill.
//  4. Mocks the latest sealed block if no start value is provided.
//  5. Subscribes using the provided subscription function.
//  6. Simulates the reception of new blocks and consumes them from the subscription channel.
//  7. Ensures that there are no new messages waiting after all blocks have been processed.
//  8. Cancels the subscription and ensures it shuts down gracefully.
func (s *BackendBlocksSuite) subscribe(
	subscribeFn func(ctx context.Context, startValue interface{}, blockStatus flow.BlockStatus) subscription_old.Subscription,
	requireFn func(interface{}, *flow.Block),
	tests []testType,
) {
	for _, test := range tests {
		s.Run(test.name, func() {
			synctest.Test(s.T(), func(t *testing.T) {
				// the broadcaster must be setup inside of the synctest bubble otherwise the test
				// will panic.
				var err error
				broadcaster := engine.NewBroadcaster()
				s.backend, err = New(s.backendParams(broadcaster))
				s.Require().NoError(err)

				// add "backfill" block - blocks that are already in the database before the test starts
				// this simulates a subscription on a past block
				if test.highestBackfill > 0 {
					s.setupBlockTrackerMock(test.blockStatus, s.blocksArray[test.highestBackfill].ToHeader())
				}

				subCtx, subCancel := context.WithCancel(context.Background())

				// mock latest sealed if no start value provided
				if test.startValue == nil {
					s.snapshot.On("Head").Unset()
					s.snapshot.On("Head").Return(s.rootBlock.ToHeader(), nil).Once()
				}

				sub := subscribeFn(subCtx, test.startValue, test.blockStatus)

				// loop over all blocks
				for i, b := range test.expectedBlocks {
					// simulate new block received.
					// all blocks with index <= highestBackfill were already received
					if i > test.highestBackfill {
						s.setupBlockTrackerMock(test.blockStatus, b.ToHeader())

						broadcaster.Publish()
					}

					// block until there is data waiting in the subscription channel
					synctest.Wait()

					// consume block from subscription
					v, ok := <-sub.Channel()
					s.Require().True(ok, "channel closed while waiting for exec data for block %x %v: err: %v", b.Height, b.ID(), sub.Err())

					requireFn(v, b)
				}

				// block until the subscription goroutine stops
				synctest.Wait()

				// make sure there are no new messages waiting. the channel should be opened with nothing waiting
				select {
				case <-sub.Channel():
					s.T().Error("expected channel to be empty")
				default:
				}

				// stop the subscription
				subCancel()

				// block until the subscription goroutine stops
				synctest.Wait()

				// ensure subscription shuts down gracefully
				v, ok := <-sub.Channel()
				s.Nil(v)
				s.False(ok)
				s.ErrorIs(sub.Err(), context.Canceled)
			})
		})
	}
}

// requireBlocks ensures that the received block information matches the expected data.
func (s *BackendBlocksSuite) requireBlocks(v interface{}, expectedBlock *flow.Block) {
	actualBlock, ok := v.(*flow.Block)
	require.True(s.T(), ok, "unexpected response type: %T", v)

	s.Require().Equalf(expectedBlock.Height, actualBlock.Height, "expected block height %d, got %d", expectedBlock.Height, actualBlock.Height)
	s.Require().Equal(expectedBlock.ID(), actualBlock.ID())
	s.Require().Equal(*expectedBlock, *actualBlock)
}

// TestSubscribeBlocksHandlesErrors tests error handling scenarios for the SubscribeBlocksFromStartBlockID and SubscribeBlocksFromStartHeight methods in the Backend.
// It ensures that the method correctly returns errors for various invalid input cases.
//
// Test Cases:
//
// 1. Returns error for unindexed start block id:
//   - Tests that subscribing to block headers with an unindexed start block ID results in a NotFound error.
//
// 2. Returns error for start height before root height:
//   - Validates that attempting to subscribe to block headers with a start height before the root height results in an InvalidArgument error.
//
// 3. Returns error for unindexed start height:
//   - Tests that subscribing to block headers with an unindexed start height results in a NotFound error.
//
// Each test case checks for specific error conditions and ensures that the methods responds appropriately.
func (s *BackendBlocksSuite) TestSubscribeBlocksHandlesErrors() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	backend, err := New(s.backendParams(engine.NewBroadcaster()))
	s.Require().NoError(err)

	s.Run("returns error if unknown start block id is provided", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := backend.SubscribeBlocksFromStartBlockID(subCtx, unittest.IdentifierFixture(), flow.BlockStatusFinalized)
		s.Equal(codes.NotFound, status.Code(sub.Err()), "expected %s, got %v: %v", codes.NotFound, status.Code(sub.Err()), sub.Err())
	})

	s.Run("returns error for start height before root height", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := backend.SubscribeBlocksFromStartHeight(subCtx, s.rootBlock.Height-1, flow.BlockStatusFinalized)
		s.Equal(codes.InvalidArgument, status.Code(sub.Err()), "expected %s, got %v: %v", codes.InvalidArgument, status.Code(sub.Err()), sub.Err())
	})

	s.Run("returns error if unknown start height is provided", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := backend.SubscribeBlocksFromStartHeight(subCtx, s.blocksArray[len(s.blocksArray)-1].Height+10, flow.BlockStatusFinalized)
		s.Equal(codes.NotFound, status.Code(sub.Err()), "expected %s, got %v: %v", codes.NotFound, status.Code(sub.Err()), sub.Err())
	})
}
