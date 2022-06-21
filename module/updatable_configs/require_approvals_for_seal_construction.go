package updatable_configs

import (
	"fmt"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/updatable_configs/validation"
)

// min number of approvals required for constructing a candidate seal
type requiredApprovalsForSealConstructionInstance struct {
	requiredApprovalsForSealConstruction *atomic.Uint32
	requiredApprovalsForSealVerification uint
	chunkAlpha                           uint
}

func NewRequiredApprovalsForSealConstructionInstance(
	requiredApprovalsForSealConstruction uint,
	requiredApprovalsForSealVerification uint,
	chunkAlpha uint,
) (module.RequiredApprovalsForSealConstructionInstanceSetter, error) {
	err := validation.ValidateRequireApprovals(
		requiredApprovalsForSealConstruction,
		requiredApprovalsForSealVerification,
		chunkAlpha,
	)
	if err != nil {
		return nil, fmt.Errorf("can not create RequiredApprovalsForSealConstructionInstance: %w", err)
	}
	return &requiredApprovalsForSealConstructionInstance{
		requiredApprovalsForSealConstruction: atomic.NewUint32(uint32(requiredApprovalsForSealConstruction)),
		requiredApprovalsForSealVerification: requiredApprovalsForSealVerification,
		chunkAlpha:                           chunkAlpha,
	}, nil
}

// SetValue updates the requiredApprovalsForSealConstruction and return the old value
// This assumes the caller has validated the new value
func (r *requiredApprovalsForSealConstructionInstance) SetValue(requiredApprovalsForSealConstruction uint) (uint, error) {
	err := validation.ValidateRequireApprovals(
		requiredApprovalsForSealConstruction,
		r.requiredApprovalsForSealVerification,
		r.chunkAlpha,
	)
	if err != nil {
		return 0, err
	}

	from := uint(r.requiredApprovalsForSealConstruction.Load())
	r.requiredApprovalsForSealConstruction.Store(uint32(requiredApprovalsForSealConstruction))

	return from, nil
}

// GetValue gets the requiredApprovalsForSealConstruction
func (r *requiredApprovalsForSealConstructionInstance) GetValue() uint {
	return uint(r.requiredApprovalsForSealConstruction.Load())
}

// ChunkAlphaConst returns the constant chunk alpha value
func (r *requiredApprovalsForSealConstructionInstance) ChunkAlphaConst() uint {
	return r.chunkAlpha
}

// RequireApprovalsForSealVerificationConst returns the constant RequireApprovalsForSealVerification value
func (r *requiredApprovalsForSealConstructionInstance) RequireApprovalsForSealVerificationConst() uint {
	return r.requiredApprovalsForSealVerification
}
