package testmocks

import "github.com/dapperlabs/flow-go/pkg/types"

func NewMockEffectsSealWithDifferentReceipt(m *MockEffectsHappyPath) *MockEffectsSealWithDifferentReceipt {
	return &MockEffectsSealWithDifferentReceipt{m, 0}
}

// MockEffectsSealWithDifferentReceipt implements the processor.Effects & Mock interfaces
type MockEffectsSealWithDifferentReceipt struct {
	*MockEffectsHappyPath
	_callCount int
}

func (m *MockEffectsSealWithDifferentReceipt) IsSealedWithDifferentReceipt(*types.ExecutionReceipt) (bool, error) {
	m._callCount++
	return true, nil
}

func (m *MockEffectsSealWithDifferentReceipt) CallCountIsSealedWithDifferentReceipt() int {
	return m._callCount
}
