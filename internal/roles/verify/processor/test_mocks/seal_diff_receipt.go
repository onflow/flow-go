package testmocks

import (
	"github.com/dapperlabs/bamboo-node/internal/pkg/types"
)

func NewMockEffectsSealWithDifferentReceipt(m *MockEffectsHappyPath) *MockEffectsSealWithDifferentReceipt {
	return &MockEffectsSealWithDifferentReceipt{m, 0}
}

// MockEffectsSealWithDifferentReceipt implements the processor.Effects & Mock interfaces
type MockEffectsSealWithDifferentReceipt struct {
	*MockEffectsHappyPath
	_callCountIsSealedWithDifferentReceipt int
}

func (m *MockEffectsSealWithDifferentReceipt) IsSealedWithDifferentReceipt(*types.ExecutionReceipt) (bool, error) {
	m._callCountIsSealedWithDifferentReceipt++
	return true, nil
}

func (m *MockEffectsSealWithDifferentReceipt) CallCountIsSealedWithDifferentReceipt() int {
	return m._callCountIsSealedWithDifferentReceipt
}
