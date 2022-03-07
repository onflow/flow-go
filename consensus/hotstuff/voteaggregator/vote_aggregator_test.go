package voteaggregator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestVoteAggregator(t *testing.T) {
	suite.Run(t, new(VoteAggregatorTestSuite))
}

// VoteCollectorsTestSuite is a test suite for isolated testing of VoteCollectors.
// Contains helper methods and mocked state which is used to verify correct behavior of VoteCollectors.
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

func (s *VoteAggregatorTestSuite) TestOnFinalizedBlock() {
	finalizedBlock := unittest.BlockHeaderFixture(unittest.HeaderWithView(100))
	done := atomic.NewBool(false)
	s.collectors.On("PruneUpToView", finalizedBlock.View).Run(func(mock.Arguments) {
		done.Toggle()
	}).Once()
	s.aggregator.OnFinalizedBlock(model.BlockFromFlow(&finalizedBlock, finalizedBlock.View-1))
	require.Eventually(s.T(), done.Load, time.Second, time.Millisecond*20)
	s.collectors.AssertCalled(s.T(), "PruneUpToView", finalizedBlock.View)
}
