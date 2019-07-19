package testmocks

import (
	"github.com/dapperlabs/bamboo-node/internal/pkg/types"
	"github.com/dapperlabs/bamboo-node/internal/roles/verify/compute"
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
