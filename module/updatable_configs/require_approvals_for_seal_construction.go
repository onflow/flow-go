package updatable_configs

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/module/updatable_configs/validation"
)

// min number of approvals required for constructing a candidate seal
type RequiredApprovalsForSealConstructionInstance struct {
	sync.RWMutex
	requiredApprovalsForSealConstruction uint
	requiredApprovalsForSealVerification uint
	chunkAlpha                           uint
}

func NewRequiredApprovalsForSealConstructionInstance(
	requiredApprovalsForSealConstruction uint,
	requiredApprovalsForSealVerification uint,
	chunkAlpha uint,
) (*RequiredApprovalsForSealConstructionInstance, error) {
	err := validation.ValidateRequireApprovals(
		requiredApprovalsForSealConstruction,
		requiredApprovalsForSealVerification,
		chunkAlpha,
	)
	if err != nil {
		return nil, fmt.Errorf("can not create RequiredApprovalsForSealConstructionInstance: %w", err)
	}
	return &RequiredApprovalsForSealConstructionInstance{
		requiredApprovalsForSealConstruction: requiredApprovalsForSealConstruction,
		requiredApprovalsForSealVerification: requiredApprovalsForSealVerification,
		chunkAlpha:                           chunkAlpha,
	}, nil
}

// SetValue updates the requiredApprovalsForSealConstruction and return the old value
// This assume the caller has validated the new value
func (r *RequiredApprovalsForSealConstructionInstance) SetValue(requiredApprovalsForSealConstruction uint) (uint, error) {
	r.Lock()
	defer r.Unlock()

	err := validation.ValidateRequireApprovals(
		requiredApprovalsForSealConstruction,
		r.requiredApprovalsForSealVerification,
		r.chunkAlpha,
	)
	if err != nil {
		return 0, err
	}

	from := r.requiredApprovalsForSealConstruction
	r.requiredApprovalsForSealConstruction = requiredApprovalsForSealConstruction

	return from, nil
}

// GetValue gets the requiredApprovalsForSealConstruction
func (r *RequiredApprovalsForSealConstructionInstance) GetValue() uint {
	r.RLock()
	defer r.RUnlock()
	return r.requiredApprovalsForSealConstruction
}
