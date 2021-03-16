package sealingtracker

import (
	"encoding/json"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// SealingRecord is a record of the sealing status for a specific
// incorporated result. It holds information whether the result is sealable,
// or what is missing to be sealable.
// Not concurrency safe.
// A SealingRecord for an incorporatedResult that we do _not_ want to track
// can be represented by nil. All methods of SealingRecord accept a nil
// receiver and in this case are NoOps.
type SealingRecord struct {
	executedBlock      *flow.Header             // block the incorporated Result is for
	incorporatedResult *flow.IncorporatedResult // the incorporated result

	// sufficientApprovalsForSealing: True iff all chunks in the result have
	// sufficient approvals
	sufficientApprovalsForSealing bool
	// firstUnmatchedChunkIndex: Index of first chunk that hasn't received
	// sufficient approval (ordered by chunk index). Optional value: only set
	// if SufficientApprovalsForSealing == False and nil otherwise.
	firstUnmatchedChunkIndex *uint64
	// qualifiesForEmergencySealing: True iff result qualifies for emergency
	// sealing. Optional value: only set if
	// SufficientApprovalsForSealing == False and nil otherwise.
	qualifiesForEmergencySealing *bool
	// hasMultipleReceipts: True iff there are at least 2 receipts from
	// _different_ ENs committing to the result. Optional value: only set if
	// SufficientApprovalsForSealing == True and nil otherwise.
	hasMultipleReceipts *bool
}

// String returns a json representation of the sealing record.
func (rs *SealingRecord) String() string {
	if rs == nil {
		return ""
	}

	result := rs.incorporatedResult.Result
	kvps := map[string]interface{}{
		"block_id":                         result.BlockID.String(),
		"height":                           rs.executedBlock.Height,
		"result_id":                        result.ID().String(),
		"incorporated_result_id":           rs.incorporatedResult.ID().String(),
		"number_chunks":                    len(result.Chunks),
		"sufficient_approvals_for_sealing": rs.sufficientApprovalsForSealing,
	}
	if rs.firstUnmatchedChunkIndex != nil {
		kvps["first_unmatched_chunk_index"] = *rs.firstUnmatchedChunkIndex
	}
	if rs.qualifiesForEmergencySealing != nil {
		kvps["qualifies_for_emergency_sealing"] = *rs.qualifiesForEmergencySealing
	}
	if rs.hasMultipleReceipts != nil {
		kvps["has_multiple_receipts"] = *rs.hasMultipleReceipts
	}

	bytes, err := json.Marshal(kvps)
	if err != nil {
		return fmt.Sprintf("internal error converting SealingRecord to json: %s", err.Error())
	}
	return string(bytes)
}

// SetHasMultipleReceipts specifies whether there are at least 2 receipts from
// _different_ ENs committing to the incorporated result.
func (rs *SealingRecord) SetHasMultipleReceipts(hasMultipleReceipts bool) {
	if rs == nil {
		return
	}
	rs.hasMultipleReceipts = &hasMultipleReceipts
}

// SetQualifiesForEmergencySealing specifies whether the incorporated result
// qualifies for emergency sealing
func (rs *SealingRecord) SetQualifiesForEmergencySealing(qualifiesForEmergencySealing bool) {
	if rs == nil {
		return
	}
	rs.qualifiesForEmergencySealing = &qualifiesForEmergencySealing
}

func (rs *SealingRecord) setSufficientApprovals() *SealingRecord {
	if rs != nil {
		rs.sufficientApprovalsForSealing = true
	}
	return rs
}

func (rs *SealingRecord) setInsufficientApprovals(firstUnmatchedChunkIndex uint64) *SealingRecord {
	if rs != nil {
		rs.sufficientApprovalsForSealing = false
		rs.firstUnmatchedChunkIndex = &firstUnmatchedChunkIndex
	}
	return rs
}
