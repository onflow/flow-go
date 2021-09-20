package voteaggregator

import (
	"errors"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"testing"
	"time"
)

var voteCollectorsFactoryError = errors.New("factory error")

func TestVoteAggregatorV2(t *testing.T) {
	suite.Run(t, new(VoteAggregatorV2TestSuite))
}

type VoteAggregatorV2TestSuite struct {
	suite.Suite

	mockedCollectors map[uint64]*mocks.VoteCollector

	committee  *mocks.Committee
	signer     *mocks.SignerVerifier
	notifier   *mocks.Consumer
	validator  *mocks.Validator
	collectors *mocks.VoteCollectors
	aggregator *VoteAggregatorV2
}

func (s *VoteAggregatorV2TestSuite) SetupTest() {
	s.committee = &mocks.Committee{}
	s.signer = &mocks.SignerVerifier{}
	s.notifier = &mocks.Consumer{}
	s.validator = &mocks.Validator{}
	s.collectors = &mocks.VoteCollectors{}

	s.mockedCollectors = make(map[uint64]*mocks.VoteCollector)

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

	s.aggregator, err = NewVoteAggregatorV2(unittest.Logger(), s.notifier, 0, s.committee,
		s.validator, s.signer, s.collectors)
	require.NoError(s.T(), err)

	// startup aggregator
	<-s.aggregator.Ready()
}

func (s *VoteAggregatorV2TestSuite) TearDownTest() {
	<-s.aggregator.Done()
}

// prepareMockedCollector prepares a mocked collector and stores it in map, later it will be used
// to mock behavior of vote collectors.
func (s *VoteAggregatorV2TestSuite) prepareMockedCollector(view uint64) *mocks.VoteCollector {
	collector := &mocks.VoteCollector{}
	collector.On("View").Return(view).Maybe()
	s.mockedCollectors[view] = collector
	return collector
}

// TestAddVote_DeliveryOfQueuedVotes tests if votes are being delivered for processing by queening logic
func (s *VoteAggregatorV2TestSuite) TestAddVote_DeliveryOfQueuedVotes() {
	view := uint64(1000)
	clr := s.prepareMockedCollector(view)
	votes := make([]*model.Vote, 10)
	for i := range votes {
		votes[i] = unittest.VoteFixture(unittest.WithVoteView(view))
		clr.On("AddVote", votes[i]).Once().Return(nil)
	}

	for _, vote := range votes {
		err := s.aggregator.AddVote(vote)
		require.NoError(s.T(), err)
	}

	time.Sleep(time.Millisecond * 100)

	clr.AssertExpectations(s.T())
}

// TestAddVote_StaleVote tests that stale votes(with view lower or equal to highest pruned view) are dropped
// and not processed.
func (s *VoteAggregatorV2TestSuite) TestAddVote_StaleVote() {
	view := uint64(1000)
	s.collectors.On("PruneUpToView", view).Return(nil)
	s.aggregator.PruneUpToView(view)

	// try adding votes with view lower than highest pruned
	votes := 10
	for i := 0; i < votes; i++ {
		vote := unittest.VoteFixture(unittest.WithVoteView(view))
		err := s.aggregator.AddVote(vote)
		require.NoError(s.T(), err)
	}

	time.Sleep(time.Millisecond * 100)
	s.collectors.AssertNotCalled(s.T(), "GetOrCreateCollector")
}

// TestAddBlock_ValidProposal tests that happy path processing of valid proposal finishes without errors
func (s *VoteAggregatorV2TestSuite) TestAddBlock_ValidProposal() {
	view := uint64(1000)
	clr := s.prepareMockedCollector(view)
	proposal := helper.MakeProposal(s.T(), helper.WithBlock(helper.MakeBlock(s.T(), helper.WithBlockView(view))))
	clr.On("ProcessBlock", proposal).Return(nil)
	err := s.aggregator.AddBlock(proposal)
	require.NoError(s.T(), err)
	clr.AssertExpectations(s.T())
}

// TestAddBlock_ProcessingErrors tests that all errors during block processing are correctly propagated to caller.
func (s *VoteAggregatorV2TestSuite) TestAddBlock_ProcessingErrors() {
	view := uint64(1000)
	proposal := helper.MakeProposal(s.T(), helper.WithBlock(helper.MakeBlock(s.T(), helper.WithBlockView(view))))
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
	staleProposal := helper.MakeProposal(s.T(), helper.WithBlock(helper.MakeBlock(s.T(), helper.WithBlockView(staleView))))
	*s.collectors = mocks.VoteCollectors{}
	s.collectors.On("GetOrCreateCollector", staleView).Return(nil, false, mempool.NewDecreasingPruningHeightError(""))
	err := s.aggregator.AddBlock(staleProposal)
	require.NoError(s.T(), err)
}

// TestAddBlock_StaleProposal tests that stale proposal(with view lower or equal to highest pruned view) is dropped
// and not processed.
func (s *VoteAggregatorV2TestSuite) TestAddBlock_StaleProposal() {
	view := uint64(1000)
	s.collectors.On("PruneUpToView", view).Return(nil)
	s.aggregator.PruneUpToView(view)

	proposal := helper.MakeProposal(s.T(), helper.WithBlock(helper.MakeBlock(s.T(), helper.WithBlockView(view))))
	err := s.aggregator.AddBlock(proposal)
	require.NoError(s.T(), err)
	s.collectors.AssertNotCalled(s.T(), "GetOrProcessCollector")
}
