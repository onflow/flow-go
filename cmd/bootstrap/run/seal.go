package run

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// GenerateRootSeal generates a root seal matching the input root result.
// The input is assumed to be a valid root result.
// No errors are expected during normal operation. 
func GenerateRootSeal(result *flow.ExecutionResult) (*flow.Seal, error) {
	finalState, err := result.FinalStateCommitment()
	if err != nil {
		return nil, fmt.Errorf("generating root seal failed: %w", err)
	}
	seal, err := flow.NewSeal(
		flow.UntrustedSeal{
			BlockID:                result.BlockID,
			ResultID:               result.ID(),
			FinalState:             finalState,
			AggregatedApprovalSigs: nil,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("could not construct seal: %w", err)
	}

	return seal, nil
}
