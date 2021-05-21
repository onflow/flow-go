package tracker

import (
	"encoding/json"
	"strings"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter/id"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/state/protocol"
)

// SealingTracker is an auxiliary component for tracking sealing progress.
// Its primary purpose is to decide which SealingRecords should be tracked
// and to store references to them.
// A SealingTracker is intended to track progress for a _single run_
// of the sealing algorithm, i.e. Core.CheckSealing().
// Not concurrency safe.
type SealingTracker struct {
	state      protocol.State
	isRelevant flow.IdentifierFilter
	records    []*SealingRecord
}

func NewSealingTracker(state protocol.State) *SealingTracker {
	return &SealingTracker{
		state:      state,
		isRelevant: nextUnsealedFinalizedBlock(state),
	}
}

// String converts the most relevant information from the SealingRecords
// to key-value pairs (in json format).
func (st *SealingTracker) String() string {
	rcrds := make([]string, 0, len(st.records))
	for _, r := range st.records {
		s, err := st.sealingRecord2String(r)
		if err != nil {
			continue
		}
		rcrds = append(rcrds, s)
	}
	return "[" + strings.Join(rcrds, ", ") + "]"
}

// MempoolHasNextSeal returns true iff the seals mempool contains a candidate seal
// for the next block
func (st *SealingTracker) MempoolHasNextSeal(seals mempool.IncorporatedResultSeals) bool {
	for _, nextUnsealed := range st.records {
		_, mempoolHasNextSeal := seals.ByID(nextUnsealed.IncorporatedResult.ID())
		if mempoolHasNextSeal {
			return true
		}
	}
	return false
}

// Track tracks the given SealingRecord, provided it should be tracked
// according to the SealingTracker's internal policy.
func (st *SealingTracker) Track(sealingRecord *SealingRecord) {
	executedBlockID := sealingRecord.IncorporatedResult.Result.BlockID
	if st.isRelevant(executedBlockID) {
		st.records = append(st.records, sealingRecord)
	}
}

// sealingRecord2String generates a string representation of a sealing record.
// We specifically attach this method to the SealingTracker, as it is the Tracker's
// responsibility to decide what information from the record should be captured
// and what additional details (like block height), should be added.
func (st *SealingTracker) sealingRecord2String(record *SealingRecord) (string, error) {
	result := record.IncorporatedResult.Result
	executedBlock, err := st.state.AtBlockID(result.BlockID).Head()
	if err != nil {
		return "", err
	}

	kvps := map[string]interface{}{
		"executed_block_id":                result.BlockID.String(),
		"executed_block_height":            executedBlock.Height,
		"result_id":                        result.ID().String(),
		"incorporated_result_id":           record.IncorporatedResult.ID().String(),
		"number_chunks":                    len(result.Chunks),
		"sufficient_approvals_for_sealing": record.SufficientApprovalsForSealing,
	}
	if record.firstUnmatchedChunkIndex != nil {
		kvps["first_unmatched_chunk_index"] = *record.firstUnmatchedChunkIndex
	}
	if record.qualifiesForEmergencySealing != nil {
		kvps["qualifies_for_emergency_sealing"] = *record.qualifiesForEmergencySealing
	}
	if record.hasMultipleReceipts != nil {
		kvps["has_multiple_receipts"] = *record.hasMultipleReceipts
	}

	bytes, err := json.Marshal(kvps)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// nextUnsealedFinalizedBlock determines the ID of the finalized but unsealed
// block with smallest height. It returns an Identity filter that only accepts
// the respective ID.
// In case the next unsealed block has not been finalized, we return the
// False-filter (or if we encounter any problems).
func nextUnsealedFinalizedBlock(state protocol.State) flow.IdentifierFilter {
	lastSealed, err := state.Sealed().Head()
	if err != nil {
		return id.False
	}

	nextUnsealedHeight := lastSealed.Height + 1
	nextUnsealed, err := state.AtHeight(nextUnsealedHeight).Head()
	if err != nil {
		return id.False
	}
	return id.Is(nextUnsealed.ID())
}
