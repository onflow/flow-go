package votecollector

import (
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// TODO: enable this when dependant modules are implemented
//func TestConsensusVoteCollector(t *testing.T) {
//	suite.Run(t, new(ConsensusVerifyingCollectorTestSuite))
//}

// VerifyingVoteCollectorTestSuite contains common mocking logic for testing that will be used by both consensus and collection
// vote collectors
type VerifyingVoteCollectorTestSuite struct {
	suite.Suite

	proposal           *model.Proposal
	combinedAggregator *mocks.CombinedSigAggregator
	reconstructor      *mocks.RandomBeaconReconstructor
}

func (s *VerifyingVoteCollectorTestSuite) SetupTest() {

	block := helper.MakeBlock(s.T(), helper.WithBlockView(100))
	s.proposal = &model.Proposal{
		Block:   block,
		SigData: nil,
	}
	s.combinedAggregator = &mocks.CombinedSigAggregator{}
	s.reconstructor = &mocks.RandomBeaconReconstructor{}
}

// ConsensusVerifyingCollectorTestSuite contains consensus cluster specific tests
type ConsensusVerifyingCollectorTestSuite struct {
	VerifyingVoteCollectorTestSuite

	collector *ConsensusClusterVoteCollector
}

func (s *ConsensusVerifyingCollectorTestSuite) SetupTest() {
	var err error
	s.VerifyingVoteCollectorTestSuite.SetupTest()
	s.collector, err = NewConsensusClusterVoteCollector(CollectionBase{}, s.proposal)
	require.NoError(s.T(), err)
}

// TestVerifyingVoteCollector_ProcessingStatus tests that processing status is expected
func (s *ConsensusVerifyingCollectorTestSuite) TestVerifyingVoteCollector_ProcessingStatus() {
	require.Equal(s.T(), hotstuff.VoteCollectorStatus(hotstuff.VoteCollectorStatusVerifying), s.collector.Status())
}
