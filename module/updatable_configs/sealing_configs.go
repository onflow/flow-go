package updatable_configs

import (
	"fmt"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/updatable_configs/validation"
)

// min number of approvals required for constructing a candidate seal
type sealingConfigs struct {
	requiredApprovalsForSealConstruction *atomic.Uint32
	requiredApprovalsForSealVerification uint
	chunkAlpha                           uint
	emergencySealingActive               bool   // flag which indicates if emergency sealing is active or not. NOTE: this is temporary while sealing & verification is under development
	approvalRequestsThreshold            uint64 // threshold for re-requesting approvals: min height difference between the latest finalized block and the block incorporating a result
}

var _ module.SealingConfigsSetter = (*sealingConfigs)(nil)

func NewSealingConfigs(
	requiredApprovalsForSealConstruction uint,
	requiredApprovalsForSealVerification uint,
	chunkAlpha uint,
	emergencySealingActive bool,
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
		emergencySealingActive:               emergencySealingActive,
		approvalRequestsThreshold:            flow.DefaultApprovalRequestsThreshold,
	}, nil
}

// SetRequiredApprovalsForSealingConstruction takes a new config value and updates the config
// if the new value is valid.
// Returns ValidationError if the new value results in an invalid sealing config.
func (r *sealingConfigs) SetRequiredApprovalsForSealingConstruction(requiredApprovalsForSealConstruction uint) error {
	err := validation.ValidateRequireApprovals(
		requiredApprovalsForSealConstruction,
		r.requiredApprovalsForSealVerification,
		r.chunkAlpha,
	)
	if err != nil {
		return NewValidationErrorf("invalid: %w", err)
	}

	r.requiredApprovalsForSealConstruction.Store(uint32(requiredApprovalsForSealConstruction))

	return nil
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

// EmergencySealingActiveConst returns the constant EmergencySealingActive value
func (r *sealingConfigs) EmergencySealingActiveConst() bool {
	return r.emergencySealingActive
}

func (r *sealingConfigs) ApprovalRequestsThresholdConst() uint64 {
	return r.approvalRequestsThreshold
}
