package run

import (
	"fmt"

	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
)

func GenerateRootResult(
	block *flow.Block,
	commit flow.StateCommitment,
	epochSetup *flow.EpochSetup,
	epochCommit *flow.EpochCommit,
) (*flow.ExecutionResult, error) {
	result, err := flow.NewRootExecutionResult(flow.UntrustedExecutionResult{
		PreviousResultID: flow.ZeroID,
		BlockID:          block.ID(),
		Chunks:           chunks.ChunkListFromCommit(commit),
		ServiceEvents:    []flow.ServiceEvent{epochSetup.ServiceEvent(), epochCommit.ServiceEvent()},
		ExecutionDataID:  flow.ZeroID,
	})
	if err != nil {
		return nil, fmt.Errorf("could not build root execution result: %w", err)
	}

	return result, nil
}
