package voteaggregator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
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
	consumer       *mocks.VoteAggregationConsumer
	stopAggregator context.CancelFunc
	errs           <-chan error
}

func (s *VoteAggregatorTestSuite) SetupTest() {
	var err error
	s.collectors = mocks.NewVoteCollectors(s.T())
	s.consumer = mocks.NewVoteAggregationConsumer(s.T())

	s.collectors.On("Start", mock.Anything).Once()
	unittest.ReadyDoneify(s.collectors)

	metricsCollector := metrics.NewNoopCollector()

	s.aggregator, err = NewVoteAggregator(
		unittest.Logger(),
		metricsCollector,
		metricsCollector,
		metricsCollector,
		s.consumer,
		0,
		s.collectors,
	)
	require.NoError(s.T(), err)

	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx, errs := irrecoverable.WithSignaler(ctx)
	s.stopAggregator = cancel
	s.errs = errs
	s.aggregator.Start(signalerCtx)
	unittest.RequireCloseBefore(s.T(), s.aggregator.Ready(), 100*time.Millisecond, "should close before timeout")
}

func (s *VoteAggregatorTestSuite) TearDownTest() {
	s.stopAggregator()
	unittest.RequireCloseBefore(s.T(), s.aggregator.Done(), 10*time.Second, "should close before timeout")
}

// TestOnFinalizedBlock tests if finalized block gets processed when send through `VoteAggregator`.
// Tests the whole processing pipeline.
func (s *VoteAggregatorTestSuite) TestOnFinalizedBlock() {
	finalizedBlock := unittest.BlockHeaderFixture(unittest.HeaderWithView(100))
	done := make(chan struct{})
	s.collectors.On("PruneUpToView", finalizedBlock.View).Run(func(args mock.Arguments) {
		close(done)
	}).Once()
	s.aggregator.OnFinalizedBlock(model.BlockFromFlow(finalizedBlock))
	unittest.AssertClosesBefore(s.T(), done, time.Second)
}

// TestProcessInvalidBlock tests that processing invalid block results in exception, when given as
// an input to AddBlock (only expects _valid_ blocks per API contract).
// The exception should be propagated to the VoteAggregator's internal `ComponentManager`.
func (s *VoteAggregatorTestSuite) TestProcessInvalidBlock() {
	block := helper.MakeProposal(
		helper.WithBlock(
			helper.MakeBlock(
				helper.WithBlockView(100),
			),
		),
	)
	processed := make(chan struct{})
	collector := mocks.NewVoteCollector(s.T())
	collector.On("ProcessBlock", block).Run(func(_ mock.Arguments) {
		close(processed)
	}).Return(model.InvalidProposalError{})
	s.collectors.On("GetOrCreateCollector", block.Block.View).Return(collector, true, nil).Once()

	// submit block for processing
	s.aggregator.AddBlock(block)
	unittest.RequireCloseBefore(s.T(), processed, 100*time.Millisecond, "should close before timeout")

	// expect a thrown error
	select {
	case err := <-s.errs:
		require.Error(s.T(), err)
		require.False(s.T(), model.IsInvalidProposalError(err))
	case <-time.After(100 * time.Millisecond):
		s.T().Fatalf("expected error but haven't received anything")
	}
}
