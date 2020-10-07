package spock

import (
	"fmt"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// Verifier provides functionality to verify spocks
type Verifier struct {
	protocolState protocol.ReadOnlyState

	receipts map[flow.Identifier][]*flow.ExecutionReceipt
}

// NewVerifier creates a new spock verifier
func NewVerifier(state protocol.ReadOnlyState) *Verifier {
	return &Verifier{
		protocolState: state,
		receipts:      make(map[flow.Identifier][]*flow.ExecutionReceipt),
	}
}

// AddReceipt adds a receipt into map if the spocks do not match any other receipts
// with the same result id
func (v *Verifier) AddReceipt(receipt *flow.ExecutionReceipt) error {
	resultID := receipt.ExecutionResult.ID()

	// if receipts result id does not exist in map, create an array with the receipt
	_, ok := v.receipts[resultID]
	if !ok {
		receipts := make([]*flow.ExecutionReceipt, 0)
		receipts = append(receipts, receipt)
		v.receipts[resultID] = receipts
		return nil
	}

	// check if candidate receipt spocks match spocks in map
	matched, err := v.matchReceipt(receipt)
	if err != nil {
		return fmt.Errorf("could not match receipt: %w", err)
	}

	// if not matched add to receipt array
	if !matched {
		v.receipts[resultID] = append(v.receipts[resultID], receipt)
	}

	// if matched we do nothing (transitive property of spock verification)

	return nil
}

// ClearReceipts clears all receipts for a specific resultID
func (v *Verifier) ClearReceipts(resultID flow.Identifier) bool {

	// check if entry exists
	_, ok := v.receipts[resultID]
	if !ok {
		return false
	}

	// clear receipts
	delete(v.receipts, resultID)

	return true
}

// VerifyApproval verifies an approval with all the distict receipts for the approvals
// result id and returns true if spocks match else false
func (v *Verifier) VerifyApproval(approval *flow.ResultApproval) (bool, error) {
	// find identities
	approver, err := v.protocolState.AtBlockID(approval.Body.BlockID).Identity(approval.Body.ApproverID)
	if err != nil {
		return false, fmt.Errorf("could not find approver identity")
	}

	receipts := v.receipts[approval.Body.ExecutionResultID]
	for _, receipt := range receipts {
		executor, err := v.protocolState.AtBlockID(receipt.ExecutionResult.BlockID).Identity(receipt.ExecutorID)
		if err != nil {
			return false, fmt.Errorf("could not find executor identity")
		}

		// verify spock
		verified, err := crypto.SPOCKVerify(approver.StakingPubKey, approval.Body.Spock, executor.StakingPubKey, receipt.Spocks[approval.Body.ChunkIndex])
		if err != nil {
			return false, fmt.Errorf("could not verify spocks: %w", err)
		}
		if verified {
			return true, nil
		}
	}

	return false, nil
}

func (v *Verifier) matchReceipt(receipt *flow.ExecutionReceipt) (bool, error) {

	unmatchedReceipts := v.receipts[receipt.ExecutionResult.ID()]
	matchedReceipt := false

	// get idenitity of candidate receipt
	identity, err := v.protocolState.AtBlockID(receipt.ExecutionResult.BlockID).Identity(receipt.ExecutorID)
	if err != nil {
		if protocol.IsIdentityNotFound(err) {
			return false, engine.NewInvalidInputErrorf("could not get executor identity: %w", err)
		}
		// unknown exception
		return false, fmt.Errorf("could not get executor identity: %w", err)
	}

	// for through each of the receipts to check for possible match of spocks
	// all the spocks in a receipt will have to match in order to be counted as
	// matched
MatchingReceiptsLoop:
	for _, u := range unmatchedReceipts {
		// get receipt identity to get public key
		uIdentity, err := v.protocolState.AtBlockID(u.ExecutionResult.BlockID).Identity(u.ExecutorID)
		if err != nil {
			if protocol.IsIdentityNotFound(err) {
				return false, engine.NewInvalidInputErrorf("could not get executor identity: %w", err)
			}
			// unknown exception
			return false, fmt.Errorf("could not get executor identity: %w", err)
		}

		// attempt to match every spock in the receipt with the candidate receipt
		// if not verified then skip receipt
		for _, chunk := range receipt.ExecutionResult.Chunks {
			// check if spocks match
			verified, err := crypto.SPOCKVerify(identity.StakingPubKey, receipt.Spocks[chunk.Index], uIdentity.StakingPubKey, u.Spocks[chunk.Index])
			if err != nil {
				return false, fmt.Errorf("could not verify spocks: %w", err)
			}
			if !verified {
				continue MatchingReceiptsLoop
			}
		}

		// all spocks matched so we should exit for loop
		// since all spocks match we dont need to add this into an array
		matchedReceipt = true
		break
	}

	return matchedReceipt, nil
}
