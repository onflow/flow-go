package validation

import (
	"fmt"
)

func ValidateRequireApprovals(requiredApprovalsForSealConstruction uint, requiredApprovalsForSealVerification uint, chunkAlpha uint) error {
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
