package votecollector

import (
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	mockhotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

// VoteProcessorTestSuiteBase is a helper structure which implements common logic between staking and combined vote
// processor test suites.
type VoteProcessorTestSuiteBase struct {
	suite.Suite

	sigWeight          uint64
	stakingTotalWeight uint64
	onQCCreatedState   mock.Mock

	stakingAggregator *mockhotstuff.WeightedSignatureAggregator
	minRequiredWeight uint64
	proposal          *model.Proposal
}

func (s *VoteProcessorTestSuiteBase) SetupTest() {
	s.stakingAggregator = &mockhotstuff.WeightedSignatureAggregator{}
	s.proposal = helper.MakeProposal()

	// let's assume we have 19 nodes each with weight 100
	s.sigWeight = 100
	s.minRequiredWeight = 1300 // we require at least 13 sigs to collect min weight
	s.stakingTotalWeight = 0

	// setup staking signature aggregator
	s.stakingAggregator.On("TrustedAdd", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		s.stakingTotalWeight += s.sigWeight
	}).Return(func(signerID flow.Identifier, sig crypto.Signature) uint64 {
		return s.stakingTotalWeight
	}, func(signerID flow.Identifier, sig crypto.Signature) error {
		return nil
	}).Maybe()
	s.stakingAggregator.On("TotalWeight").Return(func() uint64 {
		return s.stakingTotalWeight
	}).Maybe()
}

// onQCCreated is a special function that registers call in mocked state.
// ATTENTION: don't change name of this function since the same name is used in:
// s.onQCCreatedState.On("onQCCreated") statements
func (s *VoteProcessorTestSuiteBase) onQCCreated(qc *flow.QuorumCertificate) {
	s.onQCCreatedState.Called(qc)
}
