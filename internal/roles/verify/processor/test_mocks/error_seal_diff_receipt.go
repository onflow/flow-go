package testmocks

import (
	"errors"

	"github.com/dapperlabs/bamboo-node/pkg/types"
)

func NewMockEffectsErrorSealWithDifferentReceipt(m *MockEffectsHappyPath) *MockEffectsErrorSealWithDifferentReceipt {
	return &MockEffectsErrorSealWithDifferentReceipt{m, 0}
}

// MockEffectsErrorSealWithDifferentReceipt implements the processor.Effects & Mock interfaces
type MockEffectsErrorSealWithDifferentReceipt struct {
	*MockEffectsHappyPath
	_callCount int
}

func (m *MockEffectsErrorSealWithDifferentReceipt) IsSealedWithDifferentReceipt(*types.ExecutionReceipt) (bool, error) {
	m._callCount++
	return true, errors.New("IsSealedWithDifferentReceipt error")
}

func (m *MockEffectsErrorSealWithDifferentReceipt) CallCountIsSealedWithDifferentReceipt() int {
	return m._callCount
}
