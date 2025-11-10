package backend

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine"
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
	subscribeFunc := func(
		ctx context.Context,
		startValue interface{},
		blockStatus flow.BlockStatus,
	) subscription.Subscription[*flow.BlockDigest] {
		return s.backend.SubscribeBlockDigestsFromStartBlockID(ctx, startValue.(flow.Identifier), blockStatus)
	}

	subscribe(&s.BackendBlocksSuite, subscribeFunc, s.requireBlockDigests, s.subscribeFromStartBlockIdTestCases())
}

// TestSubscribeBlockDigestsFromStartHeight tests the SubscribeBlockDigestsFromStartHeight method.
func (s *BackendBlockDigestSuite) TestSubscribeBlockDigestsFromStartHeight() {
	call := func(
		ctx context.Context,
		startValue interface{},
		blockStatus flow.BlockStatus,
	) subscription.Subscription[*flow.BlockDigest] {
		return s.backend.SubscribeBlockDigestsFromStartHeight(ctx, startValue.(uint64), blockStatus)
	}

	subscribe(&s.BackendBlocksSuite, call, s.requireBlockDigests, s.subscribeFromStartHeightTestCases())
}

// TestSubscribeBlockDigestsFromLatest tests the SubscribeBlockDigestsFromLatest method.
func (s *BackendBlockDigestSuite) TestSubscribeBlockDigestsFromLatest() {
	call := func(
		ctx context.Context,
		startValue interface{},
		blockStatus flow.BlockStatus,
	) subscription.Subscription[*flow.BlockDigest] {
		return s.backend.SubscribeBlockDigestsFromLatest(ctx, blockStatus)
	}

	subscribe(&s.BackendBlocksSuite, call, s.requireBlockDigests, s.subscribeFromLatestTestCases())
}

// requireBlockDigests ensures that the received block digest information matches the expected data.
func (s *BackendBlockDigestSuite) requireBlockDigests(actualBlock *flow.BlockDigest, expectedBlock *flow.Block) {
	s.Require().Equal(expectedBlock.ID(), actualBlock.BlockID)
	s.Require().Equal(expectedBlock.Height, actualBlock.Height)
	s.Require().Equal(expectedBlock.Timestamp, uint64(actualBlock.Timestamp.UnixMilli()))
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	backend, err := New(s.backendParams(engine.NewBroadcaster()))
	s.Require().NoError(err)

	s.Run("returns error if unknown start block id is provided", func() {
		subCtx, subCancel := context.WithTimeout(ctx, 2*time.Second)
		defer subCancel()

		sub := backend.SubscribeBlockDigestsFromStartBlockID(subCtx, unittest.IdentifierFixture(), flow.BlockStatusFinalized)
		assert.Equal(s.T(), codes.NotFound, status.Code(sub.Err()), "expected %s, got %v: %v", codes.NotFound, status.Code(sub.Err()).String(), sub.Err())
	})

	s.Run("returns error for start height before root height", func() {
		subCtx, subCancel := context.WithTimeout(ctx, 2*time.Second)
		defer subCancel()

		sub := backend.SubscribeBlockDigestsFromStartHeight(subCtx, s.rootBlock.Height-1, flow.BlockStatusFinalized)
		assert.Equal(s.T(), codes.InvalidArgument, status.Code(sub.Err()), "expected %s, got %v: %v", codes.InvalidArgument, status.Code(sub.Err()).String(), sub.Err())
	})

	s.Run("returns error if unknown start height is provided", func() {
		subCtx, subCancel := context.WithTimeout(ctx, 2*time.Second)
		defer subCancel()

		sub := backend.SubscribeBlockDigestsFromStartHeight(subCtx, s.blocksArray[len(s.blocksArray)-1].Height+10, flow.BlockStatusFinalized)
		assert.Equal(s.T(), codes.NotFound, status.Code(sub.Err()), "expected %s, got %v: %v", codes.NotFound, status.Code(sub.Err()).String(), sub.Err())
	})
}
