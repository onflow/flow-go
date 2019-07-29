package testmocks

import "github.com/dapperlabs/bamboo-node/internal/roles/verify/processor"

func NewMockEffectsSealWithDifferentReceipt(m *MockEffectsHappyPath) *MockEffectsSealWithDifferentReceipt {
	return &MockEffectsSealWithDifferentReceipt{m, 0}
}

// MockEffectsSealWithDifferentReceipt implements the processor.Effects & Mock interfaces
type MockEffectsSealWithDifferentReceipt struct {
	*MockEffectsHappyPath
	_callCount int
}

func (m *MockEffectsSealWithDifferentReceipt) IsSealedWithDifferentReceipt(*processor.ExecutionReceipt) (bool, error) {
	m._callCount++
	return true, nil
}

func (m *MockEffectsSealWithDifferentReceipt) CallCountIsSealedWithDifferentReceipt() int {
	return m._callCount
}
