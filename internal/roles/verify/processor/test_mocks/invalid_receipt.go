package testmocks

import (
	"github.com/dapperlabs/flow-go/internal/roles/verify/compute"
	"github.com/dapperlabs/flow-go/pkg/types"
)

func NewMockEffectsInvalidReceipt(m *MockEffectsHappyPath) *MockEffectsInvalidReceipt {
	return &MockEffectsInvalidReceipt{m, 0}
}

// MockEffectsInvalidReceipt implements the processor.Effects & Mock interfaces
type MockEffectsInvalidReceipt struct {
	*MockEffectsHappyPath
	_callCount int
}

func (m *MockEffectsInvalidReceipt) IsValidExecutionReceipt(*types.ExecutionReceipt) (compute.ValidationResult, error) {
	m._callCount++
	return &compute.ValidationResultFail{}, nil
}

func (m *MockEffectsInvalidReceipt) CallCountIsValidExecutionReceipt() int {
	return m._callCount
}
