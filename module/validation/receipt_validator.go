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

// Functor that is used to retrieve parent of ExecutionResult.
type GetPreviousResult func(*flow.ExecutionResult) (*flow.ExecutionResult, error)

// receiptValidator holds all needed context for checking
// receipt validity against current protocol state.
type receiptValidator struct {
	state    protocol.State
	index    storage.Index
	results  storage.ExecutionResults
	verifier module.Verifier
}

func NewReceiptValidator(state protocol.State, index storage.Index, results storage.ExecutionResults, verifier module.Verifier) *receiptValidator {
	rv := &receiptValidator{
		state:    state,
		index:    index,
		results:  results,
		verifier: verifier,
	}

	return rv
}

func (v *receiptValidator) verifySignature(receipt *flow.ExecutionReceipt, nodeIdentity *flow.Identity) error {
	id := receipt.ID()
	valid, err := v.verifier.Verify(id[:], receipt.ExecutorSignature, nodeIdentity.StakingPubKey)
	if err != nil {
		return fmt.Errorf("failed to verify signature: %w", err)
	}

	if !valid {
		return engine.NewInvalidInputErrorf("invalid signature for (%x)", nodeIdentity.NodeID)
	}

	return nil
}

func (v *receiptValidator) verifyChunksFormat(result *flow.ExecutionResult) error {
	for index, chunk := range result.Chunks.Items() {
		if uint(index) != chunk.CollectionIndex {
			return engine.NewInvalidInputErrorf("invalid CollectionIndex, expected %d got %d", index, chunk.CollectionIndex)
		}

		if chunk.BlockID != result.BlockID {
			return engine.NewInvalidInputErrorf("invalid blockID, expected %v got %v", result.BlockID, chunk.BlockID)
		}
	}

	// we create one chunk per collection, plus the
	// system chunk. so we can check if the chunk number matches with the
	// number of guarantees plus one; this will ensure the execution receipt
	// cannot lie about having less chunks and having the remaining ones
	// approved
	requiredChunks := 1 // system chunk: must exist for block's ExecutionResult, even if block payload itself is empty

	index, err := v.index.ByBlockID(result.BlockID)
	if err != nil {
		// the mutator will always create payload index for a valid block
		return fmt.Errorf("could not find payload index for executed block %v: %w", result.BlockID, err)
	}

	requiredChunks += len(index.CollectionIDs)

	if result.Chunks.Len() != requiredChunks {
		return engine.NewInvalidInputErrorf("invalid number of chunks, expected %d got %d",
			requiredChunks, result.Chunks.Len())
	}

	return nil
}

func (v *receiptValidator) previousResult(result *flow.ExecutionResult) (*flow.ExecutionResult, error) {
	prevResult, err := v.results.ByID(result.PreviousResultID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, NewUnverifiableError(result.PreviousResultID)
		}
		return nil, err
	}
	return prevResult, nil
}

// subgraphCheck enforces that result forms a valid sub-graph:
// Let R1 be a result that references block A, and R2 be R1's parent result.
// The execution results form a valid subgraph if and only if R2 references
// A's parent.
func (v *receiptValidator) subgraphCheck(result *flow.ExecutionResult, prevResult *flow.ExecutionResult) error {
	block, err := v.state.AtBlockID(result.BlockID).Head()
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return engine.NewInvalidInputErrorf("no block found %v %w", result.BlockID, err)
		}
		return err
	}

	// validating the PreviousResultID field
	// ExecutionResult_X.PreviousResult.BlockID must equal to Block_X.ParentBlockID
	// for instance: given the following chain
	// A <- B <- C (ER_A) <- D
	// a result ER_C with `ID(ER_A)` as its ER_C.Result.PreviousResultID
	// would be invalid, because `ER_C.Result.PreviousResultID` must be ID(ER_B)
	if prevResult.BlockID != block.ParentID {
		return engine.NewInvalidInputErrorf("invalid block for previous result %v", prevResult.BlockID)
	}

	return nil
}

// resultChainCheck enforces that the end state of the parent result
// matches the current result's start state
func (v *receiptValidator) resultChainCheck(result *flow.ExecutionResult, prevResult *flow.ExecutionResult) error {
	finalState := prevResult.FinalStateCommitment()
	initialState := result.InitialStateCommit()

	if initialState != finalState {
		return engine.NewInvalidInputErrorf("execution results do not form chain: expecting init state %x, but got %x",
			finalState, initialState)
	}
	return nil
}

// Validate performs verifies that the ExecutionReceipt satisfies
// the following conditions:
// 	* is from Execution node with positive weight
//	* has valid signature
//	* chunks are in correct format
// 	* execution result has a valid parent and satisfies the subgraph check
// Returns nil if all checks passed successfully.
// Expected errors during normal operations:
// * engine.InvalidInputError
// * validation.UnverifiableError
func (v *receiptValidator) Validate(receipts []*flow.ExecutionReceipt) error {
	// lookup cache to avoid linear search when checking for previous result that is
	// part of payload
	payloadExecutionResults := make(map[flow.Identifier]*flow.ExecutionResult)
	for _, receipt := range receipts {
		payloadExecutionResults[receipt.ExecutionResult.ID()] = &receipt.ExecutionResult
	}
	// Build a functor that performs lookup first in receipts that were passed as payload and only then in
	// local storage. This is needed to handle a case when same block payload contains receipts that
	// reference each other.
	// ATTENTION: Here we assume that ER is valid, this lookup can return a result which is actually invalid.
	// Eventually invalid result will be detected and fail the whole validation.
	previousResult := func(executionResult *flow.ExecutionResult) (*flow.ExecutionResult, error) {
		prevResult, found := payloadExecutionResults[executionResult.PreviousResultID]
		if found {
			return prevResult, nil
		}

		return v.previousResult(executionResult)
	}

	for i, r := range receipts {
		err := v.validate(r, previousResult)
		if err != nil {
			// It's very important that we fail the whole validation if one of the receipts is invalid.
			// It allows us to make assumptions as stated in previous comment.
			return fmt.Errorf("could not validate receipt %v at index %d: %w", r.ID(), i, err)
		}
	}
	return nil
}

func (v *receiptValidator) validate(receipt *flow.ExecutionReceipt, getPreviousResult GetPreviousResult) error {
	identity, err := identityForNode(v.state, receipt.ExecutionResult.BlockID, receipt.ExecutorID)
	if err != nil {
		return fmt.Errorf(
			"failed to get executor identity %v at block %v: %w",
			receipt.ExecutorID,
			receipt.ExecutionResult.BlockID,
			err)
	}

	err = ensureStakedNodeWithRole(identity, flow.RoleExecution)
	if err != nil {
		return fmt.Errorf("staked node invalid: %w", err)
	}

	prevResult, err := getPreviousResult(&receipt.ExecutionResult)
	if err != nil {
		return err
	}

	err = v.verifySignature(receipt, identity)
	if err != nil {
		return fmt.Errorf("invalid receipt signature: %w", err)
	}

	err = v.verifyChunksFormat(&receipt.ExecutionResult)
	if err != nil {
		return fmt.Errorf("invalid chunks format for result %v: %w", receipt.ExecutionResult.ID(), err)
	}

	err = v.subgraphCheck(&receipt.ExecutionResult, prevResult)
	if err != nil {
		return fmt.Errorf("invalid execution result: %w", err)
	}

	err = v.resultChainCheck(&receipt.ExecutionResult, prevResult)
	if err != nil {
		return fmt.Errorf("invalid execution results chain: %w", err)
	}

	return nil
}

// check the receipt's data integrity by checking its result has
// both final statecommitment and initial statecommitment
func IntegrityCheck(receipt *flow.ExecutionReceipt) (flow.StateCommitment, flow.StateCommitment) {
	final := receipt.ExecutionResult.FinalStateCommitment()
	init := receipt.ExecutionResult.InitialStateCommit()
	return init, final
}
