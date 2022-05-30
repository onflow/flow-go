package updatable_configs

import (
	"sync"

	"github.com/onflow/flow-go/module"
)

type RequiredApprovalsForSealConstructionInstance struct {
	sync.RWMutex
	requiredApprovalsForSealConstruction uint // min number of approvals required for constructing a candidate seal
}

// DefaultRequiredApprovalsForSealConstruction is the default number of approvals required to construct a candidate seal
// for subsequent inclusion in block.
// when set to 1, it requires at least 1 approval to build a seal
// when set to 0, it can build seal without any approval
const DefaultRequiredApprovalsForSealConstruction = 1

var createRequiredApprovalsForSealConstructionOnce sync.Once
var instanceRequiredApprovalsForSealConstruction *RequiredApprovalsForSealConstructionInstance

func createRequiredApprovalsForSealConstruction() *RequiredApprovalsForSealConstructionInstance {
	createRequiredApprovalsForSealConstructionOnce.Do(func() {
		instanceRequiredApprovalsForSealConstruction = &RequiredApprovalsForSealConstructionInstance{
			requiredApprovalsForSealConstruction: DefaultRequiredApprovalsForSealConstruction,
		}
	})
	return instanceRequiredApprovalsForSealConstruction
}

// AcquireRequiredApprovalsForSealConstructionSetter always return the same singleton instance,
// which is created by the very first call
func AcquireRequiredApprovalsForSealConstructionSetter() module.RequiredApprovalsForSealConstructionInstanceSetter {
	return createRequiredApprovalsForSealConstruction()
}

// AcquireRequiredApprovalsForSealConstructionGetter always return the same singleton instance,
// which is created by the very first call
func AcquireRequiredApprovalsForSealConstructionGetter() module.RequiredApprovalsForSealConstructionInstanceGetter {
	return createRequiredApprovalsForSealConstruction()
}

// SetValue updates the requiredApprovalsForSealConstruction and return the old value
func (r *RequiredApprovalsForSealConstructionInstance) SetValue(requiredApprovalsForSealConstruction uint) uint {
	r.Lock()
	defer r.Unlock()

	from := r.requiredApprovalsForSealConstruction
	r.requiredApprovalsForSealConstruction = requiredApprovalsForSealConstruction
	return from
}

// GetValue gets the requiredApprovalsForSealConstruction
func (r *RequiredApprovalsForSealConstructionInstance) GetValue() uint {
	r.RLock()
	defer r.RUnlock()
	return r.requiredApprovalsForSealConstruction
}
