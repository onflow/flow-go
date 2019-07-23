package testmocks

import (
	"github.com/dapperlabs/bamboo-node/internal/pkg/types"
)

func NewMockEffectsNoMinStake(m *MockEffectsHappyPath) *MockEffectsNoMinStake {
	return &MockEffectsNoMinStake{m, 0}
}

// MockEffectsNoMinStake  implements the processor.Effects & Mock interfaces
type MockEffectsNoMinStake struct {
	*MockEffectsHappyPath
	_callCount int
}

func (m *MockEffectsNoMinStake) HasMinStake(*types.ExecutionReceipt) (bool, error) {
	m._callCount++
	return false, nil
}

func (m *MockEffectsNoMinStake) CallCountHasMinStake() int {
	return m._callCount
}
