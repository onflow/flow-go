package testmocks

import (
	"errors"

	"github.com/dapperlabs/bamboo-node/internal/pkg/types"
)

func NewMockEffectsErrorMinStake(m *MockEffectsHappyPath) *MockEffectsErrorMinStake {
	return &MockEffectsErrorMinStake{m, 0}
}

// MockEffectsErrorMinStake  implements the processor.Effects & Mock interfaces
type MockEffectsErrorMinStake struct {
	*MockEffectsHappyPath
	_callCount int
}

func (m *MockEffectsErrorMinStake) HasMinStake(*types.ExecutionReceipt) (bool, error) {
	m._callCount++
	return false, errors.New("Min stake error")
}

func (m *MockEffectsErrorMinStake) CallCountHasMinStake() int {
	return m._callCount
}
