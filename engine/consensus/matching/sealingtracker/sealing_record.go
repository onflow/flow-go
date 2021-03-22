package sealingtracker

import (
	"github.com/onflow/flow-go/model/flow"
)

// SealingRecord is a record of the sealing status for a specific
// incorporated result. It holds information whether the result is sealable,
// or what is missing to be sealable.
// Not concurrency safe.
type SealingRecord struct {
	// the incorporated result whose sealing status is tracked
	IncorporatedResult *flow.IncorporatedResult

	// SufficientApprovalsForSealing: True iff all chunks in the result have
	// sufficient approvals
	SufficientApprovalsForSealing bool

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

// NewRecordWithSufficientApprovals creates a sealing record for an
// incorporated result with sufficient approvals for sealing.
func NewRecordWithSufficientApprovals(ir *flow.IncorporatedResult) *SealingRecord {
	return &SealingRecord{
		IncorporatedResult:            ir,
		SufficientApprovalsForSealing: true,
	}
}

// NewRecordWithInsufficientApprovals creates a sealing record for an
// incorporated result that has insufficient approvals to be sealed.
// firstUnmatchedChunkIndex specifies the index of first chunk that
// hasn't received sufficient approval.
func NewRecordWithInsufficientApprovals(ir *flow.IncorporatedResult, firstUnmatchedChunkIndex uint64) *SealingRecord {
	return &SealingRecord{
		IncorporatedResult:            ir,
		SufficientApprovalsForSealing: false,
		firstUnmatchedChunkIndex:      &firstUnmatchedChunkIndex,
	}
}

// SetQualifiesForEmergencySealing specifies whether the incorporated result
// qualifies for emergency sealing
func (sr *SealingRecord) SetQualifiesForEmergencySealing(qualifiesForEmergencySealing bool) {
	sr.qualifiesForEmergencySealing = &qualifiesForEmergencySealing
}

// SetHasMultipleReceipts specifies whether there are at least 2 receipts from
// _different_ ENs committing to the incorporated result.
func (sr *SealingRecord) SetHasMultipleReceipts(hasMultipleReceipts bool) {
	sr.hasMultipleReceipts = &hasMultipleReceipts
}
