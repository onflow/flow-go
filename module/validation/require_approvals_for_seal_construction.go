package validation

import (
	"fmt"

	"github.com/onflow/flow-go/module/chunks"
)

// ValidateRequireApprovals validates the given value against the default of other values.
func ValidateRequireApprovals(requiredApprovalsForSealConstruction uint) error {
	// NOTE: validating against the default value. Even though we take in new values from flag, but
	// in practice we never use this flag, because it would need a coordination of all nodes updating
	// the flag, which will most likely done by updating a new image with a new default value.
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
