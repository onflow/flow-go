package validation

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module"
	mock2 "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestApprovalValidator(t *testing.T) {
	suite.Run(t, new(ApprovalValidationSuite))
}

type ApprovalValidationSuite struct {
	unittest.BaseChainSuite

	approvalValidator module.ApprovalValidator
	verifier          *mock2.Verifier
}

func (as *ApprovalValidationSuite) SetupTest() {
	as.SetupChain()
	as.verifier = &mock2.Verifier{}
	as.approvalValidator = NewApprovalValidator(as.State, as.verifier)
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

	err := as.approvalValidator.Validate(approval)
	as.Require().NoError(err, "should process a valid approval")
}

// try to submit an approval with invalid signature
func (as *ApprovalValidationSuite) TestApprovalInvalidSignature() {
	verifier := as.Identities[as.VerID]
	approval := unittest.ResultApprovalFixture(
		unittest.WithBlockID(as.UnfinalizedBlock.ID()),
		unittest.WithApproverID(as.VerID),
	)

	approvalID := approval.ID()
	as.verifier.On("Verify",
		approvalID[:],
		approval.VerifierSignature,
		verifier.StakingPubKey).Return(false, nil).Once()

	err := as.approvalValidator.Validate(approval)
	as.Require().Error(err, "should fail with invalid signature")
	as.Require().True(engine.IsInvalidInputError(err))
}

// Try to submit an approval for an unknown block.
// As the block is unknown, the ID of the sender should
// not matter as there is no block to verify it against
func (as *ApprovalValidationSuite) TestApprovalUnknownBlock() {
	originID := as.ConID
	approval := unittest.ResultApprovalFixture(unittest.WithApproverID(originID)) // generates approval for random block ID

	err := as.approvalValidator.Validate(approval)
	as.Require().Error(err, "should mark approval as unverifiable")
	as.Require().True(engine.IsUnverifiableInputError(err))
}

// try to submit an approval from a consensus node
func (as *ApprovalValidationSuite) TestOnApprovalInvalidRole() {
	originID := as.ConID
	approval := unittest.ResultApprovalFixture(
		unittest.WithBlockID(as.UnfinalizedBlock.ID()),
		unittest.WithApproverID(originID),
	)

	err := as.approvalValidator.Validate(approval)
	as.Require().Error(err, "should reject approval from wrong approver role")
	as.Require().True(engine.IsInvalidInputError(err))
}

// try to submit an approval from an unstaked approver
func (as *ApprovalValidationSuite) TestOnApprovalInvalidStake() {
	originID := as.VerID
	approval := unittest.ResultApprovalFixture(
		unittest.WithBlockID(as.UnfinalizedBlock.ID()),
		unittest.WithApproverID(originID),
	)
	as.Identities[originID].Stake = 0

	err := as.approvalValidator.Validate(approval)
	as.Require().Error(err, "should reject approval from unstaked approver")
	as.Require().True(engine.IsInvalidInputError(err))
}

// try to submit an approval for a sealed result
func (as *ApprovalValidationSuite) TestOnApprovalSealedResult() {
	originID := as.VerID
	approval := unittest.ResultApprovalFixture(
		unittest.WithBlockID(as.LatestSealedBlock.ID()),
		unittest.WithApproverID(originID),
	)

	err := as.approvalValidator.Validate(approval)
	as.Require().Error(err, "should ignore approval for sealed result")
	as.Require().True(engine.IsOutdatedInputError(err))
}

// try to submit an approval from ejected node
func (as *ApprovalValidationSuite) TestOnApprovalEjectedNode() {
	verifier := as.Identities[as.VerID]
	verifier.Ejected = true // mark as ejected
	approval := unittest.ResultApprovalFixture(
		unittest.WithBlockID(as.UnfinalizedBlock.ID()),
		unittest.WithApproverID(as.VerID),
	)

	approvalID := approval.ID()
	as.verifier.On("Verify",
		approvalID[:],
		approval.VerifierSignature,
		verifier.StakingPubKey).Return(true, nil).Once()

	err := as.approvalValidator.Validate(approval)
	as.Require().Error(err, "should fail because node is ejected")
	as.Require().True(engine.IsInvalidInputError(err))
}
