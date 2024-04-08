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
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	"github.com/onflow/flow-go/engine/access/subscription"
	subscriptionmock "github.com/onflow/flow-go/engine/access/subscription/mock"
	"github.com/onflow/flow-go/model/flow"
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

	blocks           *storagemock.Blocks
	headers          *storagemock.Headers
	blockTracker     *subscriptionmock.BlockTracker
	blockTrackerReal subscription.BlockTracker

	connectionFactory *connectionmock.ConnectionFactory

	chainID flow.ChainID

	broadcaster *engine.Broadcaster
	blocksArray []*flow.Block
	blockMap    map[uint64]*flow.Block
	rootBlock   flow.Block

	backend *Backend
}

// testType represents a test scenario for subscribing
type testType struct {
	name            string
	highestBackfill int
	startValue      interface{}
	blockStatus     flow.BlockStatus
}

func TestBackendBlocksSuite(t *testing.T) {
	suite.Run(t, new(BackendBlocksSuite))
}

// SetupTest initializes the test suite with required dependencies.
func (s *BackendBlocksSuite) SetupTest() {
	s.log = zerolog.New(zerolog.NewConsoleWriter())
	s.state = new(protocol.State)
	s.snapshot = new(protocol.Snapshot)
	header := unittest.BlockHeaderFixture()

	params := new(protocol.Params)
	params.On("SporkID").Return(unittest.IdentifierFixture(), nil)
	params.On("ProtocolVersion").Return(uint(unittest.Uint64InRange(10, 30)), nil)
	params.On("SporkRootBlockHeight").Return(header.Height, nil)
	params.On("SealedRoot").Return(header, nil)
	s.state.On("Params").Return(params)

	s.blocks = new(storagemock.Blocks)
	s.headers = new(storagemock.Headers)
	s.chainID = flow.Testnet
	s.connectionFactory = connectionmock.NewConnectionFactory(s.T())
	s.blockTracker = subscriptionmock.NewBlockTracker(s.T())

	s.broadcaster = engine.NewBroadcaster()

	blockCount := 5
	s.blockMap = make(map[uint64]*flow.Block, blockCount)
	s.blocksArray = make([]*flow.Block, 0, blockCount)

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
	}

	s.headers.On("ByBlockID", mock.AnythingOfType("flow.Identifier")).Return(
		func(blockID flow.Identifier) (*flow.Header, error) {
			for _, block := range s.blockMap {
				if block.ID() == blockID {
					return block.Header, nil
				}
			}
			return nil, storage.ErrNotFound
		},
	).Maybe()

	s.headers.On("ByHeight", mock.AnythingOfType("uint64")).Return(
		mocks.ConvertStorageOutput(
			mocks.StorageMapGetter(s.blockMap),
			func(block *flow.Block) *flow.Header { return block.Header },
		),
	).Maybe()

	s.blocks.On("ByHeight", mock.AnythingOfType("uint64")).Return(
		mocks.StorageMapGetter(s.blockMap),
	).Maybe()

	s.snapshot.On("Head").Return(s.rootBlock.Header, nil).Twice()
	s.state.On("Final").Return(s.snapshot, nil).Maybe()
	s.state.On("Sealed").Return(s.snapshot, nil).Maybe()

	var err error
	s.backend, err = New(s.backendParams())
	require.NoError(s.T(), err)

	// create real block tracker to use GetStartHeight from it, instead of mocking
	s.blockTrackerReal, err = subscription.NewBlockTracker(
		s.state,
		s.rootBlock.Header.Height,
		s.headers,
		s.broadcaster,
	)
	require.NoError(s.T(), err)
}

// backendParams returns the Params configuration for the backend.
func (s *BackendBlocksSuite) backendParams() Params {
	return Params{
		State:                    s.state,
		Blocks:                   s.blocks,
		Headers:                  s.headers,
		ChainID:                  s.chainID,
		MaxHeightRange:           DefaultMaxHeightRange,
		SnapshotHistoryLimit:     DefaultSnapshotHistoryLimit,
		AccessMetrics:            metrics.NewNoopCollector(),
		Log:                      s.log,
		TxErrorMessagesCacheSize: 1000,
		SubscriptionHandler: subscription.NewSubscriptionHandler(
			s.log,
			s.broadcaster,
			subscription.DefaultSendTimeout,
			subscription.DefaultResponseLimit,
			subscription.DefaultSendBufferSize,
		),
		BlockTracker: s.blockTracker,
	}
}

// subscribeFromStartBlockIdTestCases generates variations of testType scenarios for subscriptions
// starting from a specified block ID. It is designed to test the subscription functionality when the subscription
// starts from a custom block ID, either sealed or finalized.
func (s *BackendBlocksSuite) subscribeFromStartBlockIdTestCases() []testType {
	baseTests := []testType{
		{
			name:            "happy path - all new blocks",
			highestBackfill: -1, // no backfill
			startValue:      s.rootBlock.ID(),
		},
		{
			name:            "happy path - partial backfill",
			highestBackfill: 2, // backfill the first 3 blocks
			startValue:      s.blocksArray[0].ID(),
		},
		{
			name:            "happy path - complete backfill",
			highestBackfill: len(s.blocksArray) - 1, // backfill all blocks
			startValue:      s.blocksArray[0].ID(),
		},
		{
			name:            "happy path - start from root block by id",
			highestBackfill: len(s.blocksArray) - 1, // backfill all blocks
			startValue:      s.rootBlock.ID(),       // start from root block
		},
	}

	return s.setupBlockStatusesForTestCases(baseTests)
}

// subscribeFromStartHeightTestCases generates variations of testType scenarios for subscriptions
// starting from a specified block height. It is designed to test the subscription functionality when the subscription
// starts from a custom height, either sealed or finalized.
func (s *BackendBlocksSuite) subscribeFromStartHeightTestCases() []testType {
	baseTests := []testType{
		{
			name:            "happy path - all new blocks",
			highestBackfill: -1, // no backfill
			startValue:      s.rootBlock.Header.Height,
		},
		{
			name:            "happy path - partial backfill",
			highestBackfill: 2, // backfill the first 3 blocks
			startValue:      s.blocksArray[0].Header.Height,
		},
		{
			name:            "happy path - complete backfill",
			highestBackfill: len(s.blocksArray) - 1, // backfill all blocks
			startValue:      s.blocksArray[0].Header.Height,
		},
		{
			name:            "happy path - start from root block by id",
			highestBackfill: len(s.blocksArray) - 1,    // backfill all blocks
			startValue:      s.rootBlock.Header.Height, // start from root block
		},
	}

	return s.setupBlockStatusesForTestCases(baseTests)
}

// subscribeFromLatestTestCases generates variations of testType scenarios for subscriptions
// starting from the latest sealed block. It is designed to test the subscription functionality when the subscription
// starts from the latest available block, either sealed or finalized.
func (s *BackendBlocksSuite) subscribeFromLatestTestCases() []testType {
	baseTests := []testType{
		{
			name:            "happy path - all new blocks",
			highestBackfill: -1, // no backfill
		},
		{
			name:            "happy path - partial backfill",
			highestBackfill: 2, // backfill the first 3 blocks
		},
		{
			name:            "happy path - complete backfill",
			highestBackfill: len(s.blocksArray) - 1, // backfill all blocks
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
	s.blockTracker.On("GetHighestHeight", mock.Anything).Unset()
	s.blockTracker.On("GetHighestHeight", blockStatus).Return(highestHeader.Height, nil)

	if blockStatus == flow.BlockStatusSealed {
		s.snapshot.On("Head").Unset()
		s.snapshot.On("Head").Return(highestHeader, nil)
	}
}

// TestSubscribeBlocksFromStartBlockID tests the SubscribeBlocksFromStartBlockID method.
func (s *BackendBlocksSuite) TestSubscribeBlocksFromStartBlockID() {
	s.blockTracker.On(
		"GetStartHeightFromBlockID",
		mock.AnythingOfType("flow.Identifier"),
	).Return(func(startBlockID flow.Identifier) (uint64, error) {
		return s.blockTrackerReal.GetStartHeightFromBlockID(startBlockID)
	}, nil)

	call := func(ctx context.Context, startValue interface{}, blockStatus flow.BlockStatus) subscription.Subscription {
		return s.backend.SubscribeBlocksFromStartBlockID(ctx, startValue.(flow.Identifier), blockStatus)
	}

	s.subscribe(call, s.requireBlocks, s.subscribeFromStartBlockIdTestCases())
}

// TestSubscribeBlocksFromStartHeight tests the SubscribeBlocksFromStartHeight method.
func (s *BackendBlocksSuite) TestSubscribeBlocksFromStartHeight() {
	s.blockTracker.On(
		"GetStartHeightFromHeight",
		mock.AnythingOfType("uint64"),
	).Return(func(startHeight uint64) (uint64, error) {
		return s.blockTrackerReal.GetStartHeightFromHeight(startHeight)
	}, nil)

	call := func(ctx context.Context, startValue interface{}, blockStatus flow.BlockStatus) subscription.Subscription {
		return s.backend.SubscribeBlocksFromStartHeight(ctx, startValue.(uint64), blockStatus)
	}

	s.subscribe(call, s.requireBlocks, s.subscribeFromStartHeightTestCases())
}

// TestSubscribeBlocksFromLatest tests the SubscribeBlocksFromLatest method.
func (s *BackendBlocksSuite) TestSubscribeBlocksFromLatest() {
	s.blockTracker.On(
		"GetStartHeightFromLatest",
		mock.Anything,
	).Return(func(ctx context.Context) (uint64, error) {
		return s.blockTrackerReal.GetStartHeightFromLatest(ctx)
	}, nil)

	call := func(ctx context.Context, startValue interface{}, blockStatus flow.BlockStatus) subscription.Subscription {
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
	subscribeFn func(ctx context.Context, startValue interface{}, blockStatus flow.BlockStatus) subscription.Subscription,
	requireFn func(interface{}, *flow.Block),
	tests []testType,
) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, test := range tests {
		s.Run(test.name, func() {
			// add "backfill" block - blocks that are already in the database before the test starts
			// this simulates a subscription on a past block
			if test.highestBackfill > 0 {
				s.setupBlockTrackerMock(test.blockStatus, s.blocksArray[test.highestBackfill].Header)
			}

			subCtx, subCancel := context.WithCancel(ctx)

			// mock latest sealed if no start value provided
			if test.startValue == nil {
				s.snapshot.On("Head").Unset()
				s.snapshot.On("Head").Return(s.rootBlock.Header, nil).Once()
			}

			sub := subscribeFn(subCtx, test.startValue, test.blockStatus)

			// loop over all blocks
			for i, b := range s.blocksArray {
				s.T().Logf("checking block %d %v %d", i, b.ID(), b.Header.Height)

				// simulate new block received.
				// all blocks with index <= highestBackfill were already received
				if i > test.highestBackfill {
					s.setupBlockTrackerMock(test.blockStatus, b.Header)

					s.broadcaster.Publish()
				}

				// consume block from subscription
				unittest.RequireReturnsBefore(s.T(), func() {
					v, ok := <-sub.Channel()
					require.True(s.T(), ok, "channel closed while waiting for exec data for block %x %v: err: %v", b.Header.Height, b.ID(), sub.Err())

					requireFn(v, b)
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

// requireBlocks ensures that the received block information matches the expected data.
func (s *BackendBlocksSuite) requireBlocks(v interface{}, expectedBlock *flow.Block) {
	actualBlock, ok := v.(*flow.Block)
	require.True(s.T(), ok, "unexpected response type: %T", v)

	s.Require().Equal(expectedBlock.Header.Height, actualBlock.Header.Height)
	s.Require().Equal(expectedBlock.Header.ID(), actualBlock.Header.ID())
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

	// mock block tracker for SubscribeBlocksFromStartBlockID
	s.blockTracker.On(
		"GetStartHeightFromBlockID",
		mock.AnythingOfType("flow.Identifier"),
	).Return(func(startBlockID flow.Identifier) (uint64, error) {
		return s.blockTrackerReal.GetStartHeightFromBlockID(startBlockID)
	}, nil)

	s.Run("returns error if unknown start block id is provided", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeBlocksFromStartBlockID(subCtx, unittest.IdentifierFixture(), flow.BlockStatusFinalized)
		assert.Equal(s.T(), codes.NotFound, status.Code(sub.Err()), "expected %s, got %v: %v", codes.NotFound, status.Code(sub.Err()).String(), sub.Err())
	})

	// mock block tracker for GetStartHeightFromHeight
	s.blockTracker.On(
		"GetStartHeightFromHeight",
		mock.AnythingOfType("uint64"),
	).Return(func(startHeight uint64) (uint64, error) {
		return s.blockTrackerReal.GetStartHeightFromHeight(startHeight)
	}, nil)

	s.Run("returns error for start height before root height", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeBlocksFromStartHeight(subCtx, s.rootBlock.Header.Height-1, flow.BlockStatusFinalized)
		assert.Equal(s.T(), codes.InvalidArgument, status.Code(sub.Err()), "expected %s, got %v: %v", codes.InvalidArgument, status.Code(sub.Err()).String(), sub.Err())
	})

	s.Run("returns error if unknown start height is provided", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeBlocksFromStartHeight(subCtx, s.blocksArray[len(s.blocksArray)-1].Header.Height+10, flow.BlockStatusFinalized)
		assert.Equal(s.T(), codes.NotFound, status.Code(sub.Err()), "expected %s, got %v: %v", codes.NotFound, status.Code(sub.Err()).String(), sub.Err())
	})
}
