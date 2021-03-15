package sealingtracker

import (
	"encoding/json"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// SealingRecord is a record of the sealing status for a specific
// incorporated Result. It holds information whether the result is sealable,
// or what is missing to be sealable.
type SealingRecord struct {
	ExecutedBlock      *flow.Header             // block the incorporated Result is for
	IncorporatedResult *flow.IncorporatedResult // the incorporated result

	// SufficientApprovalsForSealing: True iff all chunks in the result have
	// sufficient approvals
	SufficientApprovalsForSealing bool
	// FirstUnmatchedChunkIndex: Index of first chunk that hasn't received
	// sufficient approval (ordered by chunk index). Optional value: only set
	// if SufficientApprovalsForSealing == False and nil otherwise.
	FirstUnmatchedChunkIndex *uint64
	// QualifiesForEmergencySealing: True iff result qualifies for emergency
	// sealing. Optional value: only set if
	// SufficientApprovalsForSealing == False and nil otherwise.
	QualifiesForEmergencySealing *bool
	// HasHasMultipleReceipts: True iff there are at least 2 receipts from
	// _different_ ENs committing to the result. Optional value: only set if
	// SufficientApprovalsForSealing == True and nil otherwise.
	HasMultipleReceipts *bool
}

func (rs *SealingRecord) String() string {
	if rs == nil {
		return ""
	}
	bytes, err := json.Marshal(rs)
	if err != nil {
		return fmt.Sprintf("can not convert next unsealeds to json: %s", err.Error())
	}
	return string(bytes)
}

// SetHasMultipleReceipts specifies whether there are at least 2 receipts from
// _different_ ENs committing to the incorporated result.
func (rs *SealingRecord) SetHasMultipleReceipts(hasMultipleReceipts bool) {
	if rs == nil {
		return
	}
	rs.HasMultipleReceipts = &hasMultipleReceipts
}

// SetQualifiesForEmergencySealing specifies whether the incorporated result
// qualifies for emergency sealing
func (rs *SealingRecord) SetQualifiesForEmergencySealing(qualifiesForEmergencySealing bool) {
	if rs == nil {
		return
	}
	rs.QualifiesForEmergencySealing = &qualifiesForEmergencySealing
}

func (rs *SealingRecord) setSufficientApprovals() *SealingRecord {
	if rs != nil {
		rs.SufficientApprovalsForSealing = true
	}
	return rs
}

func (rs *SealingRecord) setInsufficientApprovals(firstUnmatchedChunkIndex uint64) *SealingRecord {
	if rs != nil {
		rs.SufficientApprovalsForSealing = false
		rs.FirstUnmatchedChunkIndex = &firstUnmatchedChunkIndex
	}
	return rs
}
