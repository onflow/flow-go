package run

import (
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
)

func GenerateRootResult(
	block *flow.Block,
	commit flow.StateCommitment,
	epochSetup *flow.EpochSetup,
	epochCommit *flow.EpochCommit,
) *flow.ExecutionResult {

	result := &flow.ExecutionResult{
		PreviousResultID: flow.ZeroID,
		BlockID:          block.ID(),
		Chunks:           chunks.ChunkListFromCommit(commit),
		ServiceEvents:    []flow.ServiceEvent{epochSetup.ServiceEvent(), epochCommit.ServiceEvent()},
	}
	return result
}
