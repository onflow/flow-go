package backend

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

type BackendBlockDigestSuite struct {
	BackendBlocksSuite
}

func TestBackendBlockDigestSuite(t *testing.T) {
	suite.Run(t, new(BackendBlockDigestSuite))
}

// SetupTest initializes the test suite with required dependencies.
func (s *BackendBlockDigestSuite) SetupTest() {
	s.BackendBlocksSuite.SetupTest()
}

// TestSubscribeBlockDigestsFromStartBlockID tests the SubscribeBlockDigestsFromStartBlockID method.
func (s *BackendBlockDigestSuite) TestSubscribeBlockDigestsFromStartBlockID() {
	s.blockTracker.On(
		"GetStartHeightFromBlockID",
		mock.AnythingOfType("flow.Identifier"),
	).Return(func(startBlockID flow.Identifier) (uint64, error) {
		return s.blockTrackerReal.GetStartHeightFromBlockID(startBlockID)
	}, nil)

	call := func(ctx context.Context, startValue interface{}, blockStatus flow.BlockStatus) subscription.Subscription {
		return s.backend.SubscribeBlockDigestsFromStartBlockID(ctx, startValue.(flow.Identifier), blockStatus)
	}

	s.subscribe(call, s.requireBlockDigests, s.subscribeFromStartBlockIdTestCases())
}

// TestSubscribeBlockDigestsFromStartHeight tests the SubscribeBlockDigestsFromStartHeight method.
func (s *BackendBlockDigestSuite) TestSubscribeBlockDigestsFromStartHeight() {
	s.blockTracker.On(
		"GetStartHeightFromHeight",
		mock.AnythingOfType("uint64"),
	).Return(func(startHeight uint64) (uint64, error) {
		return s.blockTrackerReal.GetStartHeightFromHeight(startHeight)
	}, nil)

	call := func(ctx context.Context, startValue interface{}, blockStatus flow.BlockStatus) subscription.Subscription {
		return s.backend.SubscribeBlockDigestsFromStartHeight(ctx, startValue.(uint64), blockStatus)
	}

	s.subscribe(call, s.requireBlockDigests, s.subscribeFromStartHeightTestCases())
}

// TestSubscribeBlockDigestsFromLatest tests the SubscribeBlockDigestsFromLatest method.
func (s *BackendBlockDigestSuite) TestSubscribeBlockDigestsFromLatest() {
	s.blockTracker.On(
		"GetStartHeightFromLatest",
		mock.Anything,
	).Return(func(ctx context.Context) (uint64, error) {
		return s.blockTrackerReal.GetStartHeightFromLatest(ctx)
	}, nil)

	call := func(ctx context.Context, startValue interface{}, blockStatus flow.BlockStatus) subscription.Subscription {
		return s.backend.SubscribeBlockDigestsFromLatest(ctx, blockStatus)
	}

	s.subscribe(call, s.requireBlockDigests, s.subscribeFromLatestTestCases())
}

// requireBlockDigests ensures that the received block digest information matches the expected data.
func (s *BackendBlockDigestSuite) requireBlockDigests(v interface{}, expectedBlock *flow.Block) {
	actualBlock, ok := v.(*flow.BlockDigest)
	require.True(s.T(), ok, "unexpected response type: %T", v)

	s.Require().Equal(expectedBlock.Header.ID(), actualBlock.ID())
	s.Require().Equal(expectedBlock.Header.Height, actualBlock.Height)
	s.Require().Equal(expectedBlock.Header.Timestamp, actualBlock.Timestamp)
}

// TestSubscribeBlockDigestsHandlesErrors tests error handling scenarios for the SubscribeBlockDigestsFromStartBlockID and SubscribeBlockDigestsFromStartHeight methods in the Backend.
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
func (s *BackendBlockDigestSuite) TestSubscribeBlockDigestsHandlesErrors() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// mock block tracker for GetStartHeightFromBlockID
	s.blockTracker.On(
		"GetStartHeightFromBlockID",
		mock.AnythingOfType("flow.Identifier"),
	).Return(func(startBlockID flow.Identifier) (uint64, error) {
		return s.blockTrackerReal.GetStartHeightFromBlockID(startBlockID)
	}, nil)

	s.Run("returns error if unknown start block id is provided", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeBlockDigestsFromStartBlockID(subCtx, unittest.IdentifierFixture(), flow.BlockStatusFinalized)
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

		sub := s.backend.SubscribeBlockDigestsFromStartHeight(subCtx, s.rootBlock.Header.Height-1, flow.BlockStatusFinalized)
		assert.Equal(s.T(), codes.InvalidArgument, status.Code(sub.Err()), "expected %s, got %v: %v", codes.InvalidArgument, status.Code(sub.Err()).String(), sub.Err())
	})

	s.Run("returns error if unknown start height is provided", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeBlockDigestsFromStartHeight(subCtx, s.blocksArray[len(s.blocksArray)-1].Header.Height+10, flow.BlockStatusFinalized)
		assert.Equal(s.T(), codes.NotFound, status.Code(sub.Err()), "expected %s, got %v: %v", codes.NotFound, status.Code(sub.Err()).String(), sub.Err())
	})
}
