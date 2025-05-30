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

	return flow.NewExecutionResult(
		flow.ZeroID,
		block.ID(),
		chunks.ChunkListFromCommit(commit),
		[]flow.ServiceEvent{epochSetup.ServiceEvent(), epochCommit.ServiceEvent()},
		flow.ZeroID,
	)
}
