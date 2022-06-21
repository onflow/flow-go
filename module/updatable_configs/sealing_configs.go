package updatable_configs

import (
	"fmt"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/updatable_configs/validation"
)

// min number of approvals required for constructing a candidate seal
type sealingConfigs struct {
	requiredApprovalsForSealConstruction *atomic.Uint32
	requiredApprovalsForSealVerification uint
	chunkAlpha                           uint
}

var _ module.SealingConfigsSetter = (*sealingConfigs)(nil)

func NewSealingConfigs(
	requiredApprovalsForSealConstruction uint,
	requiredApprovalsForSealVerification uint,
	chunkAlpha uint,
) (module.SealingConfigsSetter, error) {
	err := validation.ValidateRequireApprovals(
		requiredApprovalsForSealConstruction,
		requiredApprovalsForSealVerification,
		chunkAlpha,
	)
	if err != nil {
		return nil, fmt.Errorf("can not create RequiredApprovalsForSealConstructionInstance: %w", err)
	}
	return &sealingConfigs{
		requiredApprovalsForSealConstruction: atomic.NewUint32(uint32(requiredApprovalsForSealConstruction)),
		requiredApprovalsForSealVerification: requiredApprovalsForSealVerification,
		chunkAlpha:                           chunkAlpha,
	}, nil
}

// SetRequiredApprovalsForSealingConstruction takes a new value and returns the old value
// if the new value is valid.  otherwise returns an error,
// and the value is not updated (equivalent to no-op)
func (r *sealingConfigs) SetRequiredApprovalsForSealingConstruction(requiredApprovalsForSealConstruction uint) (uint, error) {
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

// RequireApprovalsForSealConstructionDynamicValue gets the latest value of requiredApprovalsForSealConstruction
func (r *sealingConfigs) RequireApprovalsForSealConstructionDynamicValue() uint {
	return uint(r.requiredApprovalsForSealConstruction.Load())
}

// ChunkAlphaConst returns the constant chunk alpha value
func (r *sealingConfigs) ChunkAlphaConst() uint {
	return r.chunkAlpha
}

// RequireApprovalsForSealVerificationConst returns the constant RequireApprovalsForSealVerification value
func (r *sealingConfigs) RequireApprovalsForSealVerificationConst() uint {
	return r.requiredApprovalsForSealVerification
}
