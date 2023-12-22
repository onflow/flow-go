package validation

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/state/fork"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// receiptValidator holds all needed context for checking
// receipt validity against the current protocol state.
type receiptValidator struct {
	headers         storage.Headers
	seals           storage.Seals
	state           protocol.State
	index           storage.Index
	results         storage.ExecutionResults
	signatureHasher hash.Hasher
}

var _ module.ReceiptValidator = (*receiptValidator)(nil)

func NewReceiptValidator(state protocol.State,
	headers storage.Headers,
	index storage.Index,
	results storage.ExecutionResults,
	seals storage.Seals,
) module.ReceiptValidator {
	rv := &receiptValidator{
		state:           state,
		headers:         headers,
		index:           index,
		results:         results,
		signatureHasher: signature.NewBLSHasher(signature.ExecutionReceiptTag),
		seals:           seals,
	}
	return rv
}

// verifySignature ensures that the given receipt has a valid signature from nodeIdentity.
// Expected errors during normal operations:
//   - engine.InvalidInputError if the signature is invalid
func (v *receiptValidator) verifySignature(receipt *flow.ExecutionReceiptMeta, nodeIdentity *flow.Identity) error {
	id := receipt.ID()
	valid, err := nodeIdentity.StakingPubKey.Verify(receipt.ExecutorSignature, id[:], v.signatureHasher)
	if err != nil { // Verify(..) returns (false,nil) for invalid signature. Any error indicates unexpected internal failure.
		return irrecoverable.NewExceptionf("failed to verify signature: %w", err)
	}
	if !valid {
		return engine.NewInvalidInputErrorf("invalid signature for (%x)", nodeIdentity.NodeID)
	}
	return nil
}

// verifyChunksFormat enforces that:
//   - chunks are indexed without any gaps starting from zero
//   - each chunk references the same blockID as the top-level execution result
//   - the execution result has the correct number of chunks in accordance with the number of collections in the executed block
//
// Expected errors during normal operations:
//   - engine.InvalidInputError if the result has malformed chunks
//   - module.UnknownBlockError when the executed block is unknown
func (v *receiptValidator) verifyChunksFormat(result *flow.ExecutionResult) error {
	for index, chunk := range result.Chunks.Items() {
		if uint(index) != chunk.CollectionIndex {
			return engine.NewInvalidInputErrorf("invalid CollectionIndex, expected %d got %d", index, chunk.CollectionIndex)
		}
		if uint64(index) != chunk.Index {
			return engine.NewInvalidInputErrorf("invalid Chunk.Index, expected %d got %d", index, chunk.CollectionIndex)
		}
		if chunk.BlockID != result.BlockID {
			return engine.NewInvalidInputErrorf("invalid blockID, expected %v got %v", result.BlockID, chunk.BlockID)
		}
	}

	// For a block containing k collections, the Flow protocol prescribes that a valid execution result
	// must contain k+1 chunks. Specifically, we have one chunk per collection plus the system chunk.
	// The system chunk must exist, even if block payload itself is empty.
	index, err := v.index.ByBlockID(result.BlockID) // returns `storage.ErrNotFound` for unknown block
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return module.NewUnknownBlockError("could not find payload index for executed block %v: %w", result.BlockID, err)
		}
		return irrecoverable.NewExceptionf("unexpected failure retrieving index for executed block %v: %w", result.BlockID, err)
	}
	requiredChunks := 1 + len(index.CollectionIDs) // one chunk per collection + 1 system chunk
	if result.Chunks.Len() != requiredChunks {
		return engine.NewInvalidInputErrorf("invalid number of chunks, expected %d got %d", requiredChunks, result.Chunks.Len())
	}
	return nil
}

// subgraphCheck enforces that result forms a valid sub-graph:
// Let R1 be a result that references block A, and R2 be R1's parent result. The
// execution results form a valid subgraph if and only if R2 references A's parent.
//
// Expected errors during normal operations:
//   - sentinel engine.InvalidInputError if result does not form a valid sub-graph
//   - module.UnknownBlockError when the executed block is unknown
func (v *receiptValidator) subgraphCheck(result *flow.ExecutionResult, prevResult *flow.ExecutionResult) error {
	block, err := v.state.AtBlockID(result.BlockID).Head() // returns
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return module.NewUnknownBlockError("executed block %v unknown: %w", result.BlockID, err)
		}
		return irrecoverable.NewExceptionf("unexpected failure retrieving executed block %v: %w", result.BlockID, err)
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
// matches the current result's start state.
// This function is side effect free. The only possible error it returns is of type
//   - engine.InvalidInputError if starting state of result is inconsistent with previous result's end state
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

// Validate verifies that the ExecutionReceipt satisfies the following conditions:
//   - is from Execution node with positive weight
//   - has valid signature
//   - chunks are in correct format
//   - execution result has a valid parent and satisfies the subgraph check
//
// In order to validate a receipt, both the executed block and the parent result
// referenced in `receipt.ExecutionResult` must be known. We return nil if all checks
// pass successfully.
//
// Expected errors during normal operations:
//   - engine.InvalidInputError if receipt violates protocol condition
//   - module.UnknownBlockError if the executed block is unknown
//   - module.UnknownResultError if the receipt's parent result is unknown
//
// All other error are potential symptoms critical internal failures, such as bugs or state corruption.
func (v *receiptValidator) Validate(receipt *flow.ExecutionReceipt) error {
	parentResult, err := v.results.ByID(receipt.ExecutionResult.PreviousResultID)
	if err != nil { // we expect `storage.ErrNotFound` in case parent result is unknown; any other error is unexpected, critical failure
		if errors.Is(err, storage.ErrNotFound) {
			return module.NewUnknownResultError("parent result %v unknown: %w", receipt.ExecutionResult.PreviousResultID, err)
		}
		return irrecoverable.NewExceptionf("unexpected exception fetching parent result: %v", receipt.ExecutionResult.PreviousResultID)
	}

	// first validate result to avoid expensive signature check in `validateReceipt` in case result is invalid.
	err = v.validateResult(&receipt.ExecutionResult, parentResult)
	if err != nil {
		return fmt.Errorf("could not validate single result %v at index: %w", receipt.ExecutionResult.ID(), err)
	}

	err = v.validateReceipt(receipt.Meta(), receipt.ExecutionResult.BlockID)
	if err != nil {
		// It's very important that we fail the whole validation if one of the receipts is invalid.
		// It allows us to make assumptions as stated in previous comment.
		return fmt.Errorf("could not validate receipt %v: %w", receipt.ID(), err)
	}

	return nil
}

// ValidatePayload verifies the ExecutionReceipts and ExecutionResults
// in the payload for compliance with the protocol:
// Receipts:
//   - are from Execution node with positive weight
//   - have valid signature
//   - chunks are in correct format
//   - no duplicates in fork
//
// Results:
//   - have valid parents and satisfy the subgraph check
//   - extend the execution tree, where the tree root is the latest
//     finalized block and only results from this fork are included
//   - no duplicates in fork
//
// Expected errors during normal operations:
//   - engine.InvalidInputError if some receipts in the candidate block violate protocol condition
//   - module.UnknownBlockError if the candidate block's _parent_ is unknown
//
// All other error are potential symptoms critical internal failures, such as bugs or state corruption.
func (v *receiptValidator) ValidatePayload(candidate *flow.Block) error {
	header := candidate.Header
	payload := candidate.Payload

	// As a prerequisite, we check that candidate's parent block is known. Otherwise, we cannot validate it.
	// This check is important to distinguish expected error cases from unexpected exceptions. By confirming
	// that the protocol state knows the parent block, we guarantee that we can successfully traverse the
	// candidate's ancestry below.
	_, err := v.headers.ByBlockID(header.ParentID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return module.NewUnknownBlockError("cannot validate receipts in block, as its parent block is unknown %v: %w", header.ParentID, err)
		}
		return irrecoverable.NewExceptionf("unexpected exception retrieving the candidate block's parent %v: %w", header.ParentID, err)
	}

	// return if nothing to validate
	if len(payload.Receipts) == 0 && len(payload.Results) == 0 {
		return nil
	}

	// Get the latest sealed result on this fork and the corresponding block,
	// whose result is sealed. This block is not necessarily finalized.
	lastSeal, err := v.seals.HighestInFork(header.ParentID)
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
		// At the beginning, we checked that candidate's parent exists in the protocol state, i.e. its
		// ancestry is known and valid. Hence, any error here is a symptom of internal state corruption.
		return irrecoverable.NewExceptionf("internal error while traversing the ancestor fork of unsealed blocks: %w", err)
	}

	// tracks the number of receipts committing to each result.
	// it's ok to only index receipts at this point, because we will perform
	// all needed checks after we have validated all results.
	receiptsByResult := payload.Receipts.GroupByResultID()

	// Validate all results that are incorporated into the payload. If one is malformed, the entire block is invalid.
	for i, result := range payload.Results {
		resultID := result.ID()

		// Every included result must be accompanied by a receipt with a corresponding `ResultID`, in the same block.
		// If a result is included without a corresponding receipt, it cannot be attributed to any executor.
		receiptsForResult := len(receiptsByResult.GetGroup(resultID))
		if receiptsForResult == 0 {
			return engine.NewInvalidInputErrorf("no receipts for result %v at index %d", resultID, i)
		}

		// check for duplicated results
		if _, isDuplicate := executionTree[resultID]; isDuplicate {
			return engine.NewInvalidInputErrorf("duplicate result %v at index %d", resultID, i)
		}

		// any result must extend the execution tree with root latestSealedResult
		prevResult, extendsTree := executionTree[result.PreviousResultID]
		if !extendsTree {
			return engine.NewInvalidInputErrorf("results %v at index %d does not extend execution tree", resultID, i)
		}

		// Result must be for block on fork.
		if _, forBlockOnFork := forkBlocks[result.BlockID]; !forBlockOnFork {
			return engine.NewInvalidInputErrorf("results %v at index %d is for block not on fork (%x)", resultID, i, result.BlockID)
		}
		// Reaching the following code implies that the executed block with ID `result.BlockID` is known to the protocol state, i.e. well formed.

		// validate result
		err = v.validateResult(result, prevResult)
		if err != nil {
			if engine.IsInvalidInputError(err) {
				return fmt.Errorf("result %v at index %d is invalid: %w", resultID, i, err)
			}
			if module.IsUnknownBlockError(err) {
				// Above, we checked that the result is for an ancestor of the candidate block. If this block or parts of it are not found, our state is corrupted
				return irrecoverable.NewExceptionf("the executed block or some of its parts were not found despite the block being already incorporated: %w", err)
			}
			return fmt.Errorf("unexpected exception while validating result %v at index %d: %w", resultID, i, err)
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
			if engine.IsInvalidInputError(err) {
				return fmt.Errorf("receipt %v at index %d failed validation: %w", receiptID, i, err)
			}
			if module.IsUnknownBlockError(err) {
				// Above, we checked that the result is for an ancestor of the candidate block. If this block or parts of it are not found, our state is corrupted
				return irrecoverable.NewExceptionf("the executed block or some of its parts were not found despite the block being already incorporated: %w", err)
			}
			return fmt.Errorf("receipt %v at index %d failed validation: %w", receiptID, i, err)
		}
	}

	return nil
}

// validateResult validates that the given result is well-formed.
// While we do not check the validity of the resulting
// state commitment,
// Expected errors during normal operations:
//   - engine.InvalidInputError if the result has malformed chunks
//   - module.UnknownBlockError if blockID does not correspond to a block known by the protocol state
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

// validateReceipt validates that the given `receipt` is a valid commitment from an Execution Node
// to some result. Specifically it enforces:
// Error returns:
//   - sentinel engine.InvalidInputError if `receipt` is invalid
//   - module.UnknownBlockError if executedBlockID is unknown
func (v *receiptValidator) validateReceipt(receipt *flow.ExecutionReceiptMeta, executedBlockID flow.Identifier) error {
	identity, err := identityForNode(v.state, executedBlockID, receipt.ExecutorID)
	if err != nil {
		return fmt.Errorf("retrieving idenity of node %v at block %v failed: %w", receipt.ExecutorID, executedBlockID, err)
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
