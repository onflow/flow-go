package sealingtracker

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter/id"
	"github.com/onflow/flow-go/state/protocol"
)

// SealingTracker is an auxiliary component for tracking sealing progress
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

// SufficientApprovals creates a sealing record for an incorporated result
// with sufficient approvals for sealing.
func (st *SealingTracker) SufficientApprovals(ir *flow.IncorporatedResult) *SealingRecord {
	return st.recordSealingStatus(ir).setSufficientApprovals()
}

// InsufficientApprovals creates a sealing record for an incorporated result
// that has insufficient approvals to be sealed. firstUnmatchedChunkIndex
// specifies the index of first chunk that hasn't received sufficient approval.
func (st *SealingTracker) InsufficientApprovals(ir *flow.IncorporatedResult, firstUnmatchedChunkIndex uint64) *SealingRecord {
	return st.recordSealingStatus(ir).setInsufficientApprovals(firstUnmatchedChunkIndex)
}

// recordSealingStatus checks whether the sealing status of the provided
// incorporatedResult should be tracked. It creates a new record, if the
// result should be tracked, or returns nil otherwise.
func (st *SealingTracker) recordSealingStatus(
	incorporatedResult *flow.IncorporatedResult,
) *SealingRecord {

	executedBlockID := incorporatedResult.Result.BlockID
	if !st.isRelevant(executedBlockID) {
		return nil
	}
	executedBlock, err := st.state.AtBlockID(executedBlockID).Head()
	if err != nil {
		return nil
	}
	record := &SealingRecord{
		ExecutedBlock:      executedBlock,
		IncorporatedResult: incorporatedResult,
	}
	st.records = append(st.records, record)
	return record
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
