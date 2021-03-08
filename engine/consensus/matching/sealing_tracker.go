package matching

import (
	"encoding/json"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter/id"
	"github.com/onflow/flow-go/state/protocol"
)

// SealingTracker is an auxiliary component for tracking sealing progress.
type SealingTracker struct {
	state      protocol.State
	isRelevant flow.IdentifierFilter
	records    []*incResultSealStat
}

func NewSealingTracker(state protocol.State) *SealingTracker {
	return &SealingTracker{
		state:      state,
		isRelevant: nextUnsealedFinalizedBlock(state),
	}
}

// RecordSealingStatus records the provided sealing status of the incorporated result
func (st *SealingTracker) RecordSealingStatus(incorporatedResult *flow.IncorporatedResult,
	firstUnmatchedChunkIndex int, // show which chunk hasn't received approval
	sufficientApprovalsForSealing bool, // if true, then it should soon go to seals mempool
	qualifiesForEmergencySealing bool, // if sealed by emergency since there are too many unsealed blocks
) {
	if !st.isRelevant(incorporatedResult.Result.BlockID) {
		return
	}
	executedBlock, err := st.state.AtBlockID(incorporatedResult.Result.BlockID).Head()
	if err != nil {
		return
	}
	record := &incResultSealStat{
		BlockID:                       incorporatedResult.Result.BlockID,
		Height:                        executedBlock.Height,
		ResultID:                      incorporatedResult.Result.ID(),
		IncorporatedResultID:          incorporatedResult.ID(),
		NumberChunks:                  len(incorporatedResult.Result.Chunks),
		FirstUnmatchedChunkIndex:      firstUnmatchedChunkIndex,
		SufficientApprovalsForSealing: sufficientApprovalsForSealing,
		QualifiesForEmergencySealing:  qualifiesForEmergencySealing,
	}
	st.records = append(st.records, record)
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

// incResultSealStat records whether an incorporated Result is sealable, or what is missing to be sealable
type incResultSealStat struct {
	IncorporatedResultID          flow.Identifier // to find seal in mempool
	BlockID                       flow.Identifier // the block of of the next unsealed block
	Height                        uint64          // the height of the block
	ResultID                      flow.Identifier // if we haven't received the result, it would be ZeroID
	NumberChunks                  int
	FirstUnmatchedChunkIndex      int  // show which chunk hasn't received approval
	SufficientApprovalsForSealing bool // if true, then it should soon go to seals mempool
	QualifiesForEmergencySealing  bool // if sealed by emergency since there are too many unsealed blocks
}

func (rs *incResultSealStat) String() string {
	bytes, err := json.Marshal(rs)
	if err != nil {
		return fmt.Sprintf("can not convert next unsealeds to json: %s", err.Error())
	}
	return string(bytes)
}
