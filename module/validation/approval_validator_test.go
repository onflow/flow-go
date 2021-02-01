package validation

import (
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module"
	mock2 "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/suite"
	"testing"
)

func TestApprovalValidator(t *testing.T) {
	suite.Run(t, new(ApprovalValidationSuite))
}

type ApprovalValidationSuite struct {
	unittest.BaseChainSuite

	approvalValidator module.ApprovalValidator
	verifier          *mock2.Verifier
}

func (s *ApprovalValidationSuite) SetupTest() {
	s.SetupChain()
	s.verifier = &mock2.Verifier{}
	s.approvalValidator = NewApprovalValidator(s.State, s.verifier)
}

// try to submit an approval for a known block
func (as *ApprovalValidationSuite) TestApprovalValid() {
	verifier := as.Identities[as.VerID]
	approval := unittest.ResultApprovalFixture(
		unittest.WithBlockID(as.UnfinalizedBlock.ID()),
		unittest.WithApproverID(as.VerID),
	)

	approvalID := approval.ID()
	as.verifier.On("Verify",
		approvalID[:],
		approval.VerifierSignature,
		verifier.StakingPubKey).Return(true, nil).Once()

	// onApproval should run to completion without throwing any errors
	err := as.approvalValidator.Validate(approval)
	as.Require().NoError(err, "should process a valid approval")
}

// Try to submit an approval for an unknown block.
// As the block is unknown, the ID of the sender should
// not matter as there is no block to verify it against
func (ms *MatchingSuite) TestApprovalUnknownBlock() {
	originID := ms.ConID
	approval := unittest.ResultApprovalFixture(unittest.WithApproverID(originID)) // generates approval for random block ID

	// Make sure the approval is added to the cache for future processing
	ms.ApprovalsPL.On("Add", approval).Return(true, nil).Once()

	// onApproval should not throw an error
	err := ms.matching.onApproval(approval.Body.ApproverID, approval)
	ms.Require().NoError(err, "should cache approvals for unknown blocks")

	ms.ApprovalsPL.AssertExpectations(ms.T())
}

// try to submit an approval from a consensus node
func (ms *MatchingSuite) TestOnApprovalInvalidRole() {
	originID := ms.ConID
	approval := unittest.ResultApprovalFixture(
		unittest.WithBlockID(ms.UnfinalizedBlock.ID()),
		unittest.WithApproverID(originID),
	)

	err := ms.matching.onApproval(originID, approval)
	ms.Require().Error(err, "should reject approval from wrong approver role")
	ms.Require().True(engine.IsInvalidInputError(err))

	ms.ApprovalsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// try to submit an approval from an unstaked approver
func (ms *MatchingSuite) TestOnApprovalInvalidStake() {
	originID := ms.VerID
	approval := unittest.ResultApprovalFixture(
		unittest.WithBlockID(ms.UnfinalizedBlock.ID()),
		unittest.WithApproverID(originID),
	)
	ms.Identities[originID].Stake = 0

	err := ms.matching.onApproval(originID, approval)
	ms.Require().Error(err, "should reject approval from unstaked approver")
	ms.Require().True(engine.IsInvalidInputError(err))

	ms.ApprovalsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// try to submit an approval for a sealed result
func (ms *MatchingSuite) TestOnApprovalSealedResult() {
	originID := ms.VerID
	approval := unittest.ResultApprovalFixture(
		unittest.WithBlockID(ms.LatestSealedBlock.ID()),
		unittest.WithApproverID(originID),
	)

	err := ms.matching.onApproval(originID, approval)
	ms.Require().NoError(err, "should ignore approval for sealed result")

	ms.ApprovalsPL.AssertNumberOfCalls(ms.T(), "Add", 0)
}

// try to submit an approval that is already in the mempool
func (ms *MatchingSuite) TestOnApprovalPendingApproval() {
	originID := ms.VerID
	approval := unittest.ResultApprovalFixture(unittest.WithApproverID(originID))

	// setup the approvals mempool to check that we attempted to add the
	// approval, and return false as if it was already in the mempool
	ms.ApprovalsPL.On("Add", approval).Return(false, nil).Once()

	// onApproval should return immediately after trying to insert the approval,
	// without throwing any errors
	err := ms.matching.onApproval(approval.Body.ApproverID, approval)
	ms.Require().NoError(err, "should ignore approval if already pending")

	ms.ApprovalsPL.AssertExpectations(ms.T())
}
