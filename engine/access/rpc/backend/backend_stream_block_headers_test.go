package backend

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

type BackendBlockHeadersSuite struct {
	BackendBlocksSuite
}

func TestBackendBlockHeadersSuite(t *testing.T) {
	suite.Run(t, new(BackendBlockHeadersSuite))
}

// SetupTest initializes the test suite with required dependencies.
func (s *BackendBlockHeadersSuite) SetupTest() {
	s.BackendBlocksSuite.SetupTest()
}

// TestSubscribeBlockHeadersFromStartBlockID tests the SubscribeBlockHeadersFromStartBlockID method.
func (s *BackendBlockHeadersSuite) TestSubscribeBlockHeadersFromStartBlockID() {
	call := func(ctx context.Context, startValue interface{}, blockStatus flow.BlockStatus) subscription.Subscription {
		return s.backend.SubscribeBlockHeadersFromStartBlockID(ctx, startValue.(flow.Identifier), blockStatus)
	}

	s.subscribe(call, s.requireBlockHeaders, s.subscribeFromStartBlockIdTestCases())
}

// TestSubscribeBlockHeadersFromStartHeight tests the SubscribeBlockHeadersFromStartHeight method.
func (s *BackendBlockHeadersSuite) TestSubscribeBlockHeadersFromStartHeight() {
	call := func(ctx context.Context, startValue interface{}, blockStatus flow.BlockStatus) subscription.Subscription {
		return s.backend.SubscribeBlockHeadersFromStartHeight(ctx, startValue.(uint64), blockStatus)
	}

	s.subscribe(call, s.requireBlockHeaders, s.subscribeFromStartHeightTestCases())
}

// TestSubscribeBlockHeadersFromLatest tests the SubscribeBlockHeadersFromLatest method.
func (s *BackendBlockHeadersSuite) TestSubscribeBlockHeadersFromLatest() {
	call := func(ctx context.Context, startValue interface{}, blockStatus flow.BlockStatus) subscription.Subscription {
		return s.backend.SubscribeBlockHeadersFromLatest(ctx, blockStatus)
	}

	s.subscribe(call, s.requireBlockHeaders, s.subscribeFromLatestTestCases())
}

// requireBlockHeaders ensures that the received block header information matches the expected data.
func (s *BackendBlockHeadersSuite) requireBlockHeaders(v interface{}, expectedBlock *flow.Block) {
	actualHeader, ok := v.(*flow.Header)
	require.True(s.T(), ok, "unexpected response type: %T", v)

	s.Require().Equal(expectedBlock.Header.Height, actualHeader.Height)
	s.Require().Equal(expectedBlock.Header.ID(), actualHeader.ID())
	s.Require().Equal(*expectedBlock.Header, *actualHeader)
}

// TestSubscribeBlockHeadersHandlesErrors tests error handling scenarios for the SubscribeBlockHeadersFromStartBlockID and SubscribeBlockHeadersFromStartHeight methods in the Backend.
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
func (s *BackendBlockHeadersSuite) TestSubscribeBlockHeadersHandlesErrors() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Run("returns error for unknown start block id is provided", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeBlockHeadersFromStartBlockID(subCtx, unittest.IdentifierFixture(), flow.BlockStatusFinalized)
		assert.Equal(s.T(), codes.NotFound, status.Code(sub.Err()), "expected %s, got %v: %v", codes.NotFound, status.Code(sub.Err()).String(), sub.Err())
	})

	s.Run("returns error if start height before root height", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeBlockHeadersFromStartHeight(subCtx, s.rootBlock.Header.Height-1, flow.BlockStatusFinalized)
		assert.Equal(s.T(), codes.InvalidArgument, status.Code(sub.Err()), "expected %s, got %v: %v", codes.InvalidArgument, status.Code(sub.Err()).String(), sub.Err())
	})

	s.Run("returns error for unknown start height is provided", func() {
		subCtx, subCancel := context.WithCancel(ctx)
		defer subCancel()

		sub := s.backend.SubscribeBlockHeadersFromStartHeight(subCtx, s.blocksArray[len(s.blocksArray)-1].Header.Height+10, flow.BlockStatusFinalized)
		assert.Equal(s.T(), codes.NotFound, status.Code(sub.Err()), "expected %s, got %v: %v", codes.NotFound, status.Code(sub.Err()).String(), sub.Err())
	})
}
