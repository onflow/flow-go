package validation

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type approvalValidator struct {
	state    protocol.State
	verifier module.Verifier
}

func NewApprovalValidator(state protocol.State, verifier module.Verifier) *approvalValidator {
	return &approvalValidator{
		state:    state,
		verifier: verifier,
	}
}

func (v *approvalValidator) Validate(approval *flow.ResultApproval) error {
	// check if we already have the block the approval pertains to
	head, err := v.state.AtBlockID(approval.Body.BlockID).Head()
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("failed to retrieve header for block %x: %w", approval.Body.BlockID, err)
		}

		return engine.NewUnverifiableInputError("no header for block: %v", approval.Body.BlockID)
	}

	// drop approval, if it is for block whose height is lower or equal to already sealed height
	sealed, err := v.state.Sealed().Head()
	if err != nil {
		return fmt.Errorf("could not find sealed block: %w", err)
	}
	if sealed.Height >= head.Height {
		return engine.NewOutdatedInputErrorf("result is for already sealed and finalized block height")
	}

	identity, err := identityForNode(v.state, head.ID(), approval.Body.ApproverID)
	if err != nil {
		return fmt.Errorf("failed to get identity for node %v: %w", approval.Body.ApproverID, err)
	}

	// Check if the approver was a staked verifier at that block.
	err = ensureStakedNodeWithRole(identity, flow.RoleVerification)
	if err != nil {
		return fmt.Errorf("approval not from authorized verifier: %w", err)
	}

	err = v.verifySignature(approval, identity)
	if err != nil {
		return fmt.Errorf("invalid approval signature: %w", err)
	}

	return nil
}

func (v *approvalValidator) verifySignature(approval *flow.ResultApproval, nodeIdentity *flow.Identity) error {
	id := approval.Body.ID()
	valid, err := v.verifier.Verify(id[:], approval.VerifierSignature, nodeIdentity.StakingPubKey)
	if err != nil {
		return fmt.Errorf("failed to verify signature: %w", err)
	}

	if !valid {
		return engine.NewInvalidInputErrorf("invalid signature for (%x)", nodeIdentity.NodeID)
	}

	return nil
}
