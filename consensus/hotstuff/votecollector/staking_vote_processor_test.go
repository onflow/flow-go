package votecollector

import (
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/suite"
	"testing"
)

func TestStakingVoteProcessor(t *testing.T) {
	suite.Run(t, new(StakingVoteProcessorTestSuite))
}

// StakingVoteProcessorTestSuite is a test suite that holds mocked state for isolated testing of StakingVoteProcessor.
type StakingVoteProcessorTestSuite struct {
	VoteProcessorTestSuiteBase

	processor *StakingVoteProcessor
}

func (s *StakingVoteProcessorTestSuite) SetupTest() {
	s.VoteProcessorTestSuiteBase.SetupTest()

	s.processor = newStakingVoteProcessor(
		unittest.Logger(),
		s.proposal.Block,
		s.stakingAggregator,
		s.onQCCreated,
		s.minRequiredStake,
	)
}
