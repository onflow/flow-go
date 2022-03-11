package voteaggregator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestVoteAggregator(t *testing.T) {
	suite.Run(t, new(VoteAggregatorTestSuite))
}

// VoteAggregatorTestSuite is a test suite for isolated testing of VoteAggregator.
// Contains mocked state which is used to verify correct behavior of VoteAggregator.
// Automatically starts and stops module.Startable in SetupTest and TearDownTest respectively.
type VoteAggregatorTestSuite struct {
	suite.Suite

	aggregator     *VoteAggregator
	collectors     *mocks.VoteCollectors
	consumer       *mocks.Consumer
	stopAggregator context.CancelFunc
}

func (s *VoteAggregatorTestSuite) SetupTest() {
	var err error
	s.collectors = &mocks.VoteCollectors{}
	s.consumer = &mocks.Consumer{}

	ready := func() <-chan struct{} {
		channel := make(chan struct{})
		close(channel)
		return channel
	}()

	done := func() <-chan struct{} {
		channel := make(chan struct{})
		close(channel)
		return channel
	}()

	s.collectors.On("Start", mock.Anything).Once()
	s.collectors.On("Ready").Return(ready).Once()
	s.collectors.On("Done").Return(done).Once()

	s.aggregator, err = NewVoteAggregator(unittest.Logger(), s.consumer, 0, s.collectors)
	require.NoError(s.T(), err)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx, _ := irrecoverable.WithSignaler(ctx)
	s.stopAggregator = cancel
	s.aggregator.Start(signalerCtx)
	unittest.RequireCloseBefore(s.T(), s.aggregator.Ready(), 100*time.Millisecond, "should close before timeout")
}

func (s *VoteAggregatorTestSuite) TearDownTest() {
	s.stopAggregator()
	unittest.RequireCloseBefore(s.T(), s.aggregator.Done(), time.Second, "should close before timeout")
}

// TestOnFinalizedBlock tests if finalized block gets processed when send through `VoteAggregator`.
// Tests the whole processing pipeline.
func (s *VoteAggregatorTestSuite) TestOnFinalizedBlock() {
	finalizedBlock := unittest.BlockHeaderFixture(unittest.HeaderWithView(100))
	s.collectors.On("PruneUpToView", finalizedBlock.View).Once()
	s.aggregator.OnFinalizedBlock(model.BlockFromFlow(&finalizedBlock, finalizedBlock.View-1))
	require.Eventually(s.T(),
		func() bool {
			return s.collectors.AssertCalled(s.T(), "PruneUpToView", finalizedBlock.View)
		}, time.Second, time.Millisecond*20)
}
