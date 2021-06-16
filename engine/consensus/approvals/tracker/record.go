package tracker

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type OptionalBool struct{ value *bool }

func (o *OptionalBool) IsSet() bool { return o.value != nil }
func (o *OptionalBool) Set(b bool)  { o.value = &b }
func (o *OptionalBool) Is(b bool) bool {
	if o.value == nil {
		return false
	}
	return *o.value == b
}

type Rec map[string]interface{}

// SealingRecord is a record of the sealing status for a specific
// incorporated result. It holds information whether the result is sealable,
// or what is missing to be sealable.
// Not concurrency safe.
type SealingRecord struct {
	*SealingObservation

	// the incorporated result whose sealing status is tracked
	IncorporatedResult *flow.IncorporatedResult

	// entries holds the individual entries of the sealing record
	entries Rec

	// sufficientApprovalsForSealing: indicates whether all chunks have sufficient approvals
	// Convention: nil if business logic did not collect this data
	sufficientApprovalsForSealing OptionalBool

	// ChunksWithMissingApprovals: chunk indices that are still lacking approvals.
	// Convention:
	// * nil: business logic did not collect this data
	// * empty (and not nil): all chunks have sufficient approvals
	// * contains elements: chunks with respective indices have insufficient approvals
	ChunksWithMissingApprovals []uint64

	// QualifiesForEmergencySealing: indicates whether result qualifies for emergency
	// Convention: nil if business logic did not collect this data
	qualifiesForEmergencySealing OptionalBool

	// RequestedApprovalsCount: how many approvals where requested for this incorporated result.
	// Convention: nil if business logic did not collect this data
	RequestedApprovalsCount *uint

	// HasMultipleReceipts: True iff there are at least 2 receipts from
	// _different_ ENs committing to the result. Optional value: only set if
	// SufficientApprovalsForSealing == True and nil otherwise.
	HasMultipleReceipts *bool
}

func (r *SealingRecord) QualifiesForEmergencySealing(emergencySealable bool) {
	r.qualifiesForEmergencySealing.Set(emergencySealable)
	r.entries["qualifies_for_emergency_sealing"] = emergencySealable
}

func (r *SealingRecord) ApprovalsMissing(chunksWithMissingApprovals map[uint64]flow.IdentifierList) {
	sufficientApprovals := len(chunksWithMissingApprovals) == 0
	r.sufficientApprovalsForSealing.Set(sufficientApprovals)
	r.entries["sufficient_approvals_for_sealing"] = sufficientApprovals
	if !sufficientApprovals {
		indices := make([]uint64, 0, len(chunksWithMissingApprovals))
		for i := range chunksWithMissingApprovals {
			indices = append(indices, i)
		}
		r.entries["chunks_with_insufficient_approvals"] = indices
	}

}

func (r *SealingRecord) ApprovalsRequested(requestCount uint) {
	r.entries["number_requested_approvals"] = requestCount
}

// Generate generates a key-value map capturing the application-submitted data
// plus auxiliary data.
func (r *SealingRecord) Generate() (Rec, error) {
	rec := make(Rec)
	for k, v := range r.entries {
		rec[k] = v
	}

	irID := r.IncorporatedResult.ID()
	result := r.IncorporatedResult.Result
	resultID := result.ID()
	executedBlock, err := r.headersDB.ByBlockID(result.BlockID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve executed block %v: %w", result.BlockID, err)
	}
	incorporatingBlock, err := r.headersDB.ByBlockID(r.IncorporatedResult.IncorporatedBlockID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve incorporating block %v: %w", r.IncorporatedResult.IncorporatedBlockID, err)
	}
	initialState, err := result.InitialStateCommit()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve initial state from result %v: %w", resultID, err)
	}
	finalizationStatus, err := r.assignmentFinalizationStatus(incorporatingBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to determine finalization status of incorporating block %v: %w", r.IncorporatedResult.IncorporatedBlockID, err)
	}

	rec["executed_block_id"] = result.BlockID.String()
	rec["executed_block_height"] = executedBlock.Height
	rec["result_id"] = resultID.String()
	rec["result_incorporated_at_height"] = incorporatingBlock.Height
	rec["incorporated_result_id"] = irID.String()
	rec["result_initial_state"] = hex.EncodeToString(initialState[:])
	rec["number_chunks"] = len(result.Chunks)

	if finalizationStatus != nil {
		rec["incorporating_block"] = *finalizationStatus
	}

	if r.sufficientApprovalsForSealing.Is(true) || r.qualifiesForEmergencySealing.Is(true) {
		_, rec["candidate_seal_in_mempool"] = r.sealsPl.ByID(irID)
		rec["multiple_receipts"], err = resultHasMultipleReceipts(r.receiptsDB, resultID, r.IncorporatedResult.Result.BlockID)
		if err != nil {
			return nil, fmt.Errorf("failed to determine whether result %v has multiple receipt: %w", resultID, err)
		}
	}

	return rec, nil
}

// assignmentFinalizationStatus check whether the verifier assignment is finalized.
// This information can only be obtained without traversing the forks, if the result
// is incorporated at a height that was already finalized.
// Convention for return values:
//  * nil if result is incorporated at an unfinalized height
//  * "finalized" if result is incorporated at a finalized block
//  * "orphaned" if result is incorporated in an orphaned block
func (r *SealingRecord) assignmentFinalizationStatus(incorporatingBlock *flow.Header) (*string, error) {
	if incorporatingBlock.Height > r.finalizedBlock.Height {
		return nil, nil // result is incorporated at an unfinalized height.
	}
	finalizedBlockAtSameHeight, err := r.headersDB.ByHeight(incorporatingBlock.Height)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve incorporating block %v: %w", r.IncorporatedResult.IncorporatedBlockID, err)
	}
	var stat string
	if finalizedBlockAtSameHeight.ID() == r.IncorporatedResult.IncorporatedBlockID {
		stat = "finalized"
	} else {
		stat = "orphaned"
	}
	return &stat, nil
}

// resultHasMultipleReceipts determines whether there are at AT LEAST 2 RECEIPTS
// from _different_ ENs committing to this result.
func resultHasMultipleReceipts(receiptsDB storage.ExecutionReceipts, resultID, executedBlockID flow.Identifier) (bool, error) {
	// get all receipts that are known for the block
	receipts, err := receiptsDB.ByBlockID(executedBlockID)
	if err != nil {
		return false, fmt.Errorf("internal error querying receipts for block %v: %w", executedBlockID, err)
	}

	// Index receipts for given incorporatedResult by their executor. In case
	// there are multiple receipts from the same executor, we keep the last one.
	receiptsForIncorporatedResults := receipts.GroupByResultID().GetGroup(resultID)
	return receiptsForIncorporatedResults.GroupByExecutorID().NumberGroups() >= 2, nil
}
