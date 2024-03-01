package backend

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/status"

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

// TestSubscribeBlockHeaders tests the functionality of the SubscribeBlockHeaders method in the Backend.
// It covers various scenarios for subscribing to block blocks, handling backfill, and receiving block header updates.
// The test cases include scenarios for both finalized and sealed block blocks.
//
// Test Cases:
//
// 1. Happy path - all new block blocks:
//   - No backfill is performed, and the subscription starts from the current root block header.
//
// 2. Happy path - partial backfill:
//   - A partial backfill is performed, simulating an ongoing subscription to the blockchain.
//
// 3. Happy path - complete backfill:
//   - A complete backfill is performed, simulating the subscription starting from a specific block header.
//
// 4. Happy path - start from root block by height:
//   - The subscription starts from the root block header, specified by height.
//
// 5. Happy path - start from root block by ID:
//   - The subscription starts from the root block header, specified by block ID.
//
// Each test case simulates the reception of new block blocks during the subscription, ensuring that the SubscribeBlockHeaders
// method correctly handles updates and delivers the expected block header information to the subscriber.
//
// Test Steps:
// - Initialize the test environment, including the Backend instance, mock components, and test data.
// - For each test case, set up the backfill, if applicable.
// - Subscribe to block blocks using the SubscribeBlockHeaders method.
// - Simulate the reception of new blocks during the subscription.
// - Validate that the received block header information matches the expected data.
// - Ensure the subscription shuts down gracefully when canceled.
func (s *BackendBlockHeadersSuite) TestSubscribeBlockHeaders() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, test := range s.tests {
		s.Run(test.name, func() {
			// add "backfill" block - blocks that are already in the database before the test starts
			// this simulates a subscription on a past block
			if test.highestBackfill > 0 {
				s.setupChainStateMock(test.blockStatus, s.blocksArray[test.highestBackfill].Header)
			}

			subCtx, subCancel := context.WithCancel(ctx)
			sub := s.backend.SubscribeBlockHeaders(subCtx, test.startBlockID, test.startHeight, test.blockStatus)

			// loop over all blocks
			for i, b := range s.blocksArray {
				s.T().Logf("checking block %d %v %d", i, b.ID(), b.Header.Height)

				// simulate new block received.
				// all blocks with index <= highestBackfill were already received
				if i > test.highestBackfill {
					s.setupChainStateMock(test.blockStatus, b.Header)

					s.broadcaster.Publish()
				}

				// consume block from subscription
				unittest.RequireReturnsBefore(s.T(), func() {
					v, ok := <-sub.Channel()
					require.True(s.T(), ok, "channel closed while waiting for exec data for block %x %v: err: %v", b.Header.Height, b.ID(), sub.Err())

					actualHeader, ok := v.(*flow.Header)
					require.True(s.T(), ok, "unexpected response type: %T", v)

					s.Require().Equal(b.Header.Height, actualHeader.Height)
					s.Require().Equal(b.Header.ID(), actualHeader.ID())
					s.Require().Equal(*b.Header, *actualHeader)
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

// TestSubscribeBlockHeadersHandlesErrors tests error handling scenarios for the SubscribeBlockHeaders method in the Backend.
// It ensures that the method correctly returns errors for various invalid input cases.
//
// Test Cases:
//
// 1. Returns error for unknown block status:
//   - Verifies that attempting to subscribe to block blocks with an unknown block status results in an InvalidArgument error.
//
// 2. Returns error if both start blockID and start height are provided:
//   - Ensures that providing both start block ID and start height results in an InvalidArgument error.
//
// 3. Returns error for start height before root height:
//   - Validates that attempting to subscribe to block blocks with a start height before the root height results in an InvalidArgument error.
//
// 4. Returns error for unindexed start blockID:
//   - Tests that subscribing to block blocks with an unindexed start block ID results in a NotFound error.
//
// 5. Returns error for unindexed start height:
//   - Tests that subscribing to block blocks with an unindexed start height results in a NotFound error.
//
// Each test case checks for specific error conditions and ensures that the SubscribeBlockHeaders method responds appropriately.
func (s *BackendBlockHeadersSuite) TestSubscribeBlockHeadersHandlesErrors() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, test := range s.errorTests {
		s.Run(test.name, func() {
			subCtx, subCancel := context.WithCancel(ctx)
			defer subCancel()

			sub := s.backend.SubscribeBlockHeaders(subCtx, test.startBlockID, test.startHeight, test.blockStatus)
			assert.Equal(s.T(), test.expectedErrorCode, status.Code(sub.Err()), "expected %s, got %v: %v", test.expectedErrorCode, status.Code(sub.Err()).String(), sub.Err())
		})
	}
}
