package validation

import (
	"fmt"

	"github.com/onflow/flow-go/module/chunks"
)

// ValidateRequireApprovals validates the given value against the default of other values.
func ValidateRequireApprovals(requiredApprovalsForSealConstruction uint) error {
	return validateRequireApprovals(
		requiredApprovalsForSealConstruction,
		DefaultRequiredApprovalsForSealValidation,
		chunks.DefaultChunkAssignmentAlpha,
	)
}

func validateRequireApprovals(requiredApprovalsForSealConstruction uint, requiredApprovalsForSealVerification uint, chunkAlpha uint) error {
	// We need to ensure `requiredApprovalsForSealVerification <= requiredApprovalsForSealConstruction <= chunkAlpha`
	if requiredApprovalsForSealVerification > requiredApprovalsForSealConstruction {
		return fmt.Errorf("invalid consensus parameters, requiredApprovalsForSealVerification (%v)  > requiredApprovalsForSealConstruction (%v)",
			requiredApprovalsForSealVerification, requiredApprovalsForSealConstruction)
	}

	if requiredApprovalsForSealConstruction > chunkAlpha {
		return fmt.Errorf("invalid consensus parameters: requiredApprovalsForSealConstruction (%v) > chunkAlpha (%v)",
			requiredApprovalsForSealConstruction, chunkAlpha)
	}

	return nil
}
