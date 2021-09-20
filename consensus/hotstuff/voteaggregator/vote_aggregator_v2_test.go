package voteaggregator

import (
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/suite"
	"testing"
)

func TestVoteAggregatorV2(t *testing.T) {
	suite.Run(t, new(VoteAggregatorV2TestSuite))
}

type VoteAggregatorV2TestSuite struct {
	suite.Suite

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

	s.aggregator = NewVoteAggregatorV2(unittest.Logger(), s.notifier, 0, s.committee,
		s.validator, s.signer, s.collectors)

	// startup aggregator
	<-s.aggregator.Ready()
}

func (s *VoteAggregatorV2TestSuite) TearDownTest() {
	<-s.aggregator.Done()
}
