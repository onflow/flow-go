package voteaggregator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/utils/unittest"
)

var voteCollectorsFactoryError = errors.New("factory error")

// TestVoteAggregatorV2 is a test suite for VoteAggregator. Tests happy and unhappy path scenarios
// for processing proposals, votes and also reporting slashing conditions when invalid proposal/vote was detected.
// Vote aggregator has a queening mechanism, tests are split into separately testing event delivery and
// event processing logic.
func TestVoteAggregatorV2(t *testing.T) {
	suite.Run(t, new(VoteAggregatorV2TestSuite))
}

// VoteAggregatorV2TestSuite is a test suite that holds needed mocked state for isolated testing of VoteAggregator.
type VoteAggregatorV2TestSuite struct {
	suite.Suite

	mockedCollectors map[uint64]*mocks.VoteCollector

	committee  *mocks.Committee
	signer     *mocks.SignerVerifier
	notifier   *mocks.Consumer
	collectors *mocks.VoteCollectors
	cancel     context.CancelFunc

	aggregator *VoteAggregator
}

func (s *VoteAggregatorV2TestSuite) SetupTest() {
	s.committee = &mocks.Committee{}
	s.signer = &mocks.SignerVerifier{}
	s.notifier = &mocks.Consumer{}
	s.collectors = &mocks.VoteCollectors{}

	s.mockedCollectors = make(map[uint64]*mocks.VoteCollector)

	s.collectors.On("Start", mock.Anything) // to support module.Startable
	s.collectors.On("GetOrCreateCollector", mock.Anything).Return(
		func(view uint64) hotstuff.VoteCollector {
			return s.mockedCollectors[view]
		}, func(view uint64) bool {
			return true
		}, func(view uint64) error {
			_, ok := s.mockedCollectors[view]
			if !ok {
				return voteCollectorsFactoryError
			}
			return nil
		})

	var err error
	s.aggregator, err = NewVoteAggregator(unittest.Logger(), s.notifier, 0, s.collectors)
	require.NoError(s.T(), err)

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	signalerCtx, _ := irrecoverable.WithSignaler(ctx)

	// startup aggregator
	s.aggregator.Start(signalerCtx)

	unittest.RequireCloseBefore(s.T(), s.aggregator.Ready(), 100*time.Millisecond, "aggregator not started")
}

func (s *VoteAggregatorV2TestSuite) TearDownTest() {
	s.cancel()
	unittest.RequireCloseBefore(s.T(), s.aggregator.Done(), 100*time.Millisecond, "aggregator not stopped")
}

// prepareMockedCollector prepares a mocked collector and stores it in map, later it will be used
// to mock behavior of vote collectors.
func (s *VoteAggregatorV2TestSuite) prepareMockedCollector(view uint64) *mocks.VoteCollector {
	collector := &mocks.VoteCollector{}
	collector.On("View").Return(view).Maybe()
	s.mockedCollectors[view] = collector
	return collector
}

// TestAddVote_DeliveryOfQueuedVotes tests if votes are being delivered for processing by queueing logic
func (s *VoteAggregatorV2TestSuite) TestAddVote_DeliveryOfQueuedVotes() {
	view := uint64(1000)
	clr := s.prepareMockedCollector(view)
	votes := make([]*model.Vote, 10)
	addVoteCalled := atomic.NewInt32(0)
	for i := range votes {
		votes[i] = unittest.VoteFixture(unittest.WithVoteView(view))
		clr.On("AddVote", votes[i]).Run(func(args mock.Arguments) {
			addVoteCalled.Add(1)
		}).Once().Return(nil)
	}

	for _, vote := range votes {
		s.aggregator.AddVote(vote)
	}

	require.Eventually(s.T(), func() bool {
		return int32(len(votes)) == addVoteCalled.Load()
	}, time.Second, time.Millisecond*20)

	clr.AssertExpectations(s.T())
}

// TestAddVote_StaleVote tests that stale votes(with view lower or equal to highest pruned view) are dropped
// and not processed.
func (s *VoteAggregatorV2TestSuite) TestAddVote_StaleVote() {
	view := uint64(1000)
	s.collectors.On("PruneUpToView", view).Return(nil).Once()
	s.aggregator.PruneUpToView(view)

	lengthObserver := func(int) {
		require.Fail(s.T(), "stale vote has to be dropped")
	}
	var err error
	s.aggregator.queuedVotes, err = fifoqueue.NewFifoQueue(fifoqueue.WithLengthObserver(lengthObserver))
	require.NoError(s.T(), err)

	// try adding votes with view lower than highest pruned
	votes := uint64(10)
	for i := uint64(0); i < votes; i++ {
		vote := unittest.VoteFixture(unittest.WithVoteView(view - i))
		s.aggregator.AddVote(vote)
	}
}

// TestAddBlock_ValidProposal tests that happy path processing of valid proposal finishes without errors
func (s *VoteAggregatorV2TestSuite) TestAddBlock_ValidProposal() {
	view := uint64(1000)
	clr := s.prepareMockedCollector(view)
	proposal := helper.MakeProposal(helper.WithBlock(helper.MakeBlock(helper.WithBlockView(view))))
	clr.On("ProcessBlock", proposal).Return(nil).Once()
	err := s.aggregator.AddBlock(proposal)
	require.NoError(s.T(), err)
	clr.AssertExpectations(s.T())
}

// TestAddBlock_ProcessingErrors tests that all errors during block processing are correctly propagated to caller.
func (s *VoteAggregatorV2TestSuite) TestAddBlock_ProcessingErrors() {
	view := uint64(1000)
	proposal := helper.MakeProposal(helper.WithBlock(helper.MakeBlock(helper.WithBlockView(view))))
	// calling AddBlock without prepared collector will result in factory error
	err := s.aggregator.AddBlock(proposal)
	require.ErrorIs(s.T(), err, voteCollectorsFactoryError)

	clr := s.prepareMockedCollector(view)
	expectedError := errors.New("processing-error")
	clr.On("ProcessBlock", proposal).Return(expectedError)
	err = s.aggregator.AddBlock(proposal)
	require.ErrorIs(s.T(), err, expectedError)
}

// TestAddBlock_DecreasingPruningHeightError tests that adding proposal while pruning in parallel thread doesn't
// trigger an error
func (s *VoteAggregatorV2TestSuite) TestAddBlock_DecreasingPruningHeightError() {
	staleView := uint64(1000)
	staleProposal := helper.MakeProposal(helper.WithBlock(helper.MakeBlock(helper.WithBlockView(staleView))))
	*s.collectors = mocks.VoteCollectors{}
	s.collectors.On("GetOrCreateCollector", staleView).Return(nil, false, mempool.NewDecreasingPruningHeightError(""))
	err := s.aggregator.AddBlock(staleProposal)
	require.Error(s.T(), err)
	require.True(s.T(), mempool.IsDecreasingPruningHeightError(err))
	s.collectors.AssertExpectations(s.T())
}

// TestAddBlock_StaleProposal tests that stale proposal(with view lower or equal to highest pruned view) is dropped
// and not processed.
func (s *VoteAggregatorV2TestSuite) TestAddBlock_StaleProposal() {
	view := uint64(1000)
	s.collectors.On("PruneUpToView", view).Return(nil).Once()
	s.aggregator.PruneUpToView(view)

	proposal := helper.MakeProposal(helper.WithBlock(helper.MakeBlock(helper.WithBlockView(view))))
	err := s.aggregator.AddBlock(proposal)
	require.Error(s.T(), err)
	require.True(s.T(), mempool.IsDecreasingPruningHeightError(err))
	s.collectors.AssertNotCalled(s.T(), "GetOrProcessCollector")
}

// TestVoteProcessing_DoubleVoting tests that double voting during vote processing is handled internally
// without being propagated to caller
func (s *VoteAggregatorV2TestSuite) TestVoteProcessing_DoubleVoting() {
	view := uint64(1000)
	clr := s.prepareMockedCollector(view)
	firstVote := unittest.VoteFixture(unittest.WithVoteView(view))
	secondVote := unittest.VoteFixture(unittest.WithVoteView(view))
	clr.On("AddVote", secondVote).Return(model.NewDoubleVoteErrorf(firstVote, secondVote, ""))
	s.notifier.On("OnDoubleVotingDetected", firstVote, secondVote).Return(nil).Once()

	err := s.aggregator.processQueuedVote(secondVote)
	// double voting should be handled internally and reported to notifier
	require.NoError(s.T(), err)
	s.notifier.AssertExpectations(s.T())
}

// TestVoteProcessing_DecreasingPruningHeightError tests that adding vote while pruning in parallel thread doesn't
// trigger an error
func (s *VoteAggregatorV2TestSuite) TestVoteProcessing_DecreasingPruningHeightError() {
	staleView := uint64(1000)
	*s.collectors = mocks.VoteCollectors{}
	s.collectors.On("GetOrCreateCollector", staleView).Return(nil, false, mempool.NewDecreasingPruningHeightError(""))
	err := s.aggregator.processQueuedVote(unittest.VoteFixture(unittest.WithVoteView(staleView)))
	require.NoError(s.T(), err)
	s.collectors.AssertExpectations(s.T())
}

// TestVoteProcessing_ProcessingExceptions tests that all unexpected errors are propagated to caller
func (s *VoteAggregatorV2TestSuite) TestVoteProcessing_ProcessingExceptions() {
	vote := unittest.VoteFixture()
	// failing to create collector is a critical error
	err := s.aggregator.processQueuedVote(vote)
	require.ErrorIs(s.T(), err, voteCollectorsFactoryError)

	view := uint64(1000)
	vote = unittest.VoteFixture(unittest.WithVoteView(view))
	clr := s.prepareMockedCollector(view)
	expectedError := errors.New("processing-error")
	clr.On("AddVote", vote).Return(expectedError).Once()
	err = s.aggregator.processQueuedVote(vote)
	require.ErrorIs(s.T(), err, expectedError)
	clr.AssertExpectations(s.T())
}

// TestInvalidBlock tests that InvalidBlock reports all previously processed votes to hotstuff.Consumer
// By definition only votes for invalid proposal has to be reported
func (s *VoteAggregatorV2TestSuite) TestInvalidBlock() {
	view := uint64(1000)
	byzProposal := helper.MakeProposal(helper.WithBlock(helper.MakeBlock(helper.WithBlockView(view))))

	votes := 10
	consumedVotes := make([]*model.Vote, 0, votes*2)
	// generate votes for two proposals
	honestBlockID := unittest.IdentifierFixture()
	for i := 0; i < votes; i++ {
		byzVote := unittest.VoteFixture(unittest.WithVoteView(view),
			unittest.WithVoteBlockID(byzProposal.Block.BlockID))
		honestVote := unittest.VoteFixture(unittest.WithVoteView(view),
			unittest.WithVoteBlockID(honestBlockID))
		consumedVotes = append(consumedVotes, byzVote, honestVote)

		// only votes for one of the proposals should be reported
		s.notifier.On("OnVoteForInvalidBlockDetected", byzVote, byzProposal)
	}

	clr := s.prepareMockedCollector(view)
	var voteConsumerCallback hotstuff.VoteConsumer
	clr.On("RegisterVoteConsumer", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		voteConsumerCallback = args.Get(0).(hotstuff.VoteConsumer)
		for _, vote := range consumedVotes {
			voteConsumerCallback(vote)
		}
	})

	err := s.aggregator.InvalidBlock(byzProposal)
	require.NoError(s.T(), err)

	clr.On("AddVote", mock.Anything).Run(func(args mock.Arguments) {
		voteConsumerCallback(args.Get(0).(*model.Vote))
	}).Return(nil)

	// generate more votes after notifying about invalid block
	for i := 0; i < votes; i++ {
		byzVote := unittest.VoteFixture(unittest.WithVoteView(view),
			unittest.WithVoteBlockID(byzProposal.Block.BlockID))
		honestVote := unittest.VoteFixture(unittest.WithVoteView(view),
			unittest.WithVoteBlockID(honestBlockID))

		// only votes for one of the proposals should be reported
		s.notifier.On("OnVoteForInvalidBlockDetected", byzVote, byzProposal).Once()
		require.NoError(s.T(), s.aggregator.processQueuedVote(byzVote))
		require.NoError(s.T(), s.aggregator.processQueuedVote(honestVote))
	}

	s.notifier.AssertExpectations(s.T())
}

// TestPruneUpToView tests behavior of pruning logic, we expect that valid pruning call be delegated to VoteCollectors
func (s *VoteAggregatorV2TestSuite) TestPruneUpToView() {
	// if we pass view which is higher than current highest pruned view then we should prune VoteCollectors
	view := uint64(1000)
	s.collectors.On("PruneUpToView", view).Return(nil).Once()
	s.aggregator.PruneUpToView(view)

	// calling with same view should result in noop since it's lower or equal than current highest pruned view
	s.aggregator.PruneUpToView(view)

	s.collectors.AssertNumberOfCalls(s.T(), "PruneUpToView", 1)
}
