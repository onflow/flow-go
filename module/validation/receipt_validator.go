package validation

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/fork"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// receiptValidator holds all needed context for checking
// receipt validity against current protocol state.
type receiptValidator struct {
	headers         storage.Headers
	seals           storage.Seals
	state           protocol.State
	index           storage.Index
	results         storage.ExecutionResults
	signatureHasher hash.Hasher
}

func NewReceiptValidator(state protocol.State,
	headers storage.Headers,
	index storage.Index,
	results storage.ExecutionResults,
	seals storage.Seals,
) *receiptValidator {
	rv := &receiptValidator{
		state:           state,
		headers:         headers,
		index:           index,
		results:         results,
		signatureHasher: crypto.NewBLSKMAC(encoding.ExecutionReceiptTag),
		seals:           seals,
	}

	return rv
}

func (v *receiptValidator) verifySignature(receipt *flow.ExecutionReceiptMeta, nodeIdentity *flow.Identity) error {
	id := receipt.ID()
	valid, err := nodeIdentity.StakingPubKey.Verify(receipt.ExecutorSignature, id[:], v.signatureHasher)
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

func (v *receiptValidator) fetchResult(resultID flow.Identifier) (*flow.ExecutionResult, error) {
	prevResult, err := v.results.ByID(resultID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, engine.NewUnverifiableInputError("cannot retrieve result: %v", resultID)
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
	finalState, err := prevResult.FinalStateCommitment()
	if err != nil {
		return fmt.Errorf("missing final state commitment in parent result %v", prevResult.ID())
	}
	initialState, err := result.InitialStateCommit()
	if err != nil {
		return engine.NewInvalidInputErrorf("missing initial state commitment in execution result %v", result.ID())
	}
	if initialState != finalState {
		return engine.NewInvalidInputErrorf("execution results do not form chain: expecting init state %x, but got %x",
			finalState, initialState)
	}
	return nil
}

// Validate verifies that the ExecutionReceipt satisfies
// the following conditions:
// 	* is from Execution node with positive weight
//	* has valid signature
//	* chunks are in correct format
// 	* execution result has a valid parent and satisfies the subgraph check
// Returns nil if all checks passed successfully.
// Expected errors during normal operations:
// * engine.InvalidInputError
//   if receipt violates protocol condition
// * engine.UnverifiableInputError
//   if receipt's parent result is unknown
func (v *receiptValidator) Validate(receipt *flow.ExecutionReceipt) error {
	// TODO: this can be optimized by checking if result was already stored and validated.
	// This needs to be addressed later since many tests depend on this behavior.
	prevResult, err := v.fetchResult(receipt.ExecutionResult.PreviousResultID)
	if err != nil {
		return fmt.Errorf("error fetching parent result of receipt %v: %w", receipt.ID(), err)
	}

	// first validate result to avoid signature check in in `validateReceipt` in case result is invalid.
	err = v.validateResult(&receipt.ExecutionResult, prevResult)
	if err != nil {
		return fmt.Errorf("could not validate single result %v at index: %w", receipt.ExecutionResult.ID(), err)
	}

	err = v.validateReceipt(receipt.Meta(), receipt.ExecutionResult.BlockID)
	if err != nil {
		// It's very important that we fail the whole validation if one of the receipts is invalid.
		// It allows us to make assumptions as stated in previous comment.
		return fmt.Errorf("could not validate single receipt %v: %w", receipt.ID(), err)
	}

	return nil
}

// ValidatePayload verifies the ExecutionReceipts and ExecutionResults
// in the payload for compliance with the protocol:
// Receipts:
// 	* are from Execution node with positive weight
//	* have valid signature
//	* chunks are in correct format
//  * no duplicates in fork
// Results:
// 	* have valid parents and satisfy the subgraph check
//  * extend the execution tree, where the tree root is the latest
//    finalized block and only results from this fork are included
//  * no duplicates in fork
// Expected errors during normal operations:
// * engine.InvalidInputError
//   if some receipts in the candidate block violate protocol condition
// * engine.UnverifiableInputError
//   if for some of the receipts, their respective parent result is unknown
func (v *receiptValidator) ValidatePayload(candidate *flow.Block) error {
	header := candidate.Header
	payload := candidate.Payload

	// return if nothing to validate
	if len(payload.Receipts) == 0 && len(payload.Results) == 0 {
		return nil
	}

	// Get the latest sealed result on this fork and the corresponding block,
	// whose result is sealed. This block is not necessarily finalized.
	lastSeal, err := v.seals.ByBlockID(header.ParentID)
	if err != nil {
		return fmt.Errorf("could not retrieve latest seal for fork with head %x: %w", header.ParentID, err)
	}
	latestSealedResult, err := v.results.ByID(lastSeal.ResultID)
	if err != nil {
		return fmt.Errorf("could not retrieve latest sealed result %x: %w", lastSeal.ResultID, err)
	}

	// forkBlocks is the set of all _unsealed_ blocks on the fork. We
	// use it to identify receipts that are for blocks not in the fork.
	forkBlocks := make(map[flow.Identifier]struct{})

	// Sub-Set of the execution tree: only contains `ExecutionResult`s that descent from latestSealedResult.
	// Used for detecting duplicates and results with invalid parent results.
	executionTree := make(map[flow.Identifier]*flow.ExecutionResult)
	executionTree[lastSeal.ResultID] = latestSealedResult

	// Set of previously included receipts. Used for detecting duplicates.
	forkReceipts := make(map[flow.Identifier]struct{})

	// Start from the lowest unsealed block and walk the chain upwards until we
	// hit the candidate's parent. For each visited block track:
	bookKeeper := func(block *flow.Header) error {
		blockID := block.ID()
		// track encountered blocks
		forkBlocks[blockID] = struct{}{}

		payloadIndex, err := v.index.ByBlockID(blockID)
		if err != nil {
			return fmt.Errorf("could not retrieve payload index: %w", err)
		}

		// track encountered receipts
		for _, recID := range payloadIndex.ReceiptIDs {
			forkReceipts[recID] = struct{}{}
		}

		// extend execution tree
		for _, resultID := range payloadIndex.ResultIDs {
			result, err := v.results.ByID(resultID)
			if err != nil {
				return fmt.Errorf("could not retrieve result %v: %w", resultID, err)
			}
			if _, ok := executionTree[result.PreviousResultID]; !ok {
				// We only collect results that directly descend from the last sealed result.
				// Because Results are listed in an order that satisfies the parent-first
				// relationship, we can skip all results whose parents are unknown.
				continue
			}
			executionTree[resultID] = result
		}
		return nil
	}
	err = fork.TraverseForward(v.headers, header.ParentID, bookKeeper, fork.ExcludingBlock(lastSeal.BlockID))
	if err != nil {
		return fmt.Errorf("internal error while traversing the ancestor fork of unsealed blocks: %w", err)
	}

	// first validate all results that were included into payload
	// if one of results is invalid we fail the whole check because it could be violating
	// parent-children relationship
	for i, result := range payload.Results {
		resultID := result.ID()

		// check for duplicated results
		if _, isDuplicate := executionTree[resultID]; isDuplicate {
			return engine.NewInvalidInputErrorf("duplicate result %v at index %d", resultID, i)
		}

		// any result must extend the execution tree with root latestSealedResult
		prevResult, extendsTree := executionTree[result.PreviousResultID]
		if !extendsTree {
			return engine.NewInvalidInputErrorf("results %v at index %d does not extend execution tree", resultID, i)
		}

		// result must be for block on fork
		if _, forBlockOnFork := forkBlocks[result.BlockID]; !forBlockOnFork {
			return engine.NewInvalidInputErrorf("results %v at index %d is for block not on fork (%x)", resultID, i, result.BlockID)
		}

		// validate result
		err = v.validateResult(result, prevResult)
		if err != nil {
			return fmt.Errorf("could not validate result %v at index %d: %w", resultID, i, err)
		}
		executionTree[resultID] = result
	}

	// check receipts:
	// * no duplicates
	// * must commit to a result in the execution tree with root latestSealedResult,
	//   but not latestSealedResult
	// It's very important that we fail the whole validation if one of the receipts is invalid.
	delete(executionTree, lastSeal.ResultID)
	for i, receipt := range payload.Receipts {
		receiptID := receipt.ID()

		// error if the result is not part of the execution tree with root latestSealedResult
		result, isForLegitimateResult := executionTree[receipt.ResultID]
		if !isForLegitimateResult {
			return engine.NewInvalidInputErrorf("receipt %v at index %d commits to unexpected result", receiptID, i)
		}

		// error if the receipt is duplicated in the fork
		if _, isDuplicate := forkReceipts[receiptID]; isDuplicate {
			return engine.NewInvalidInputErrorf("duplicate receipt %v at index %d", receiptID, i)
		}
		forkReceipts[receiptID] = struct{}{}

		err = v.validateReceipt(receipt, result.BlockID)
		if err != nil {
			return fmt.Errorf("receipt %v at index %d failed validation: %w", receiptID, i, err)
		}
	}

	return nil
}

func (v *receiptValidator) validateResult(result *flow.ExecutionResult, prevResult *flow.ExecutionResult) error {
	err := v.verifyChunksFormat(result)
	if err != nil {
		return fmt.Errorf("invalid chunks format for result %v: %w", result.ID(), err)
	}

	err = v.subgraphCheck(result, prevResult)
	if err != nil {
		return fmt.Errorf("invalid execution result: %w", err)
	}

	err = v.resultChainCheck(result, prevResult)
	if err != nil {
		return fmt.Errorf("invalid execution results chain: %w", err)
	}

	return nil
}

func (v *receiptValidator) validateReceipt(receipt *flow.ExecutionReceiptMeta, blockID flow.Identifier) error {
	identity, err := identityForNode(v.state, blockID, receipt.ExecutorID)
	if err != nil {
		return fmt.Errorf(
			"failed to get executor identity %v at block %v: %w",
			receipt.ExecutorID,
			blockID,
			err)
	}

	err = ensureNodeHasWeightAndRole(identity, flow.RoleExecution)
	if err != nil {
		return fmt.Errorf("node is not authorized execution node: %w", err)
	}

	err = v.verifySignature(receipt, identity)
	if err != nil {
		return fmt.Errorf("invalid receipt signature: %w", err)
	}

	return nil
}
