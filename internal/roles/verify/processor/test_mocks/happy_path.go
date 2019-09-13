package testmocks

import (
	"github.com/dapperlabs/flow-go/internal/roles/verify/compute"
	"github.com/dapperlabs/flow-go/pkg/types"
)

// MockEffectsHappyPath implements the processor.Effects & Mock interfaces
type MockEffectsHappyPath struct {
	_callCountIsValidExecutionReceipt      int
	_callCountHasMinStake                  int
	_callCountIsSealedWithDifferentReceipt int
	_callCountSend                         int
	_callCountSlashExpiredReceipt          int
	_callCountSlashInvalidReceipt          int
	_callCountHandleError                  int
}

func (m *MockEffectsHappyPath) IsValidExecutionReceipt(*types.ExecutionReceipt) (compute.ValidationResult, error) {
	m._callCountIsValidExecutionReceipt++
	return &compute.ValidationResultSuccess{}, nil
}

func (m *MockEffectsHappyPath) HasMinStake(*types.ExecutionReceipt) (bool, error) {
	m._callCountHasMinStake++
	return true, nil
}

func (m *MockEffectsHappyPath) IsSealedWithDifferentReceipt(*types.ExecutionReceipt) (bool, error) {
	m._callCountIsSealedWithDifferentReceipt++
	return false, nil
}

func (m *MockEffectsHappyPath) Send(*types.ExecutionReceipt, []byte) error {
	m._callCountSend++
	return nil
}

func (m *MockEffectsHappyPath) SlashExpiredReceipt(*types.ExecutionReceipt) error {
	m._callCountSlashExpiredReceipt++
	return nil
}

func (m *MockEffectsHappyPath) SlashInvalidReceipt(*types.ExecutionReceipt, *compute.BlockPartExecutionResult) error {
	m._callCountSlashInvalidReceipt++
	return nil
}

func (m *MockEffectsHappyPath) HandleError(err error) {
	m._callCountHandleError++
	return
}

func (m *MockEffectsHappyPath) CallCountIsValidExecutionReceipt() int {
	return m._callCountIsValidExecutionReceipt
}

func (m *MockEffectsHappyPath) CallCountHasMinStake() int {
	return m._callCountHasMinStake
}

func (m *MockEffectsHappyPath) CallCountIsSealedWithDifferentReceipt() int {
	return m._callCountIsSealedWithDifferentReceipt
}

func (m *MockEffectsHappyPath) CallCountSend() int {
	return m._callCountSend
}

func (m *MockEffectsHappyPath) CallCountSlashExpiredReceipt() int {
	return m._callCountSlashExpiredReceipt
}

func (m *MockEffectsHappyPath) CallCountSlashInvalidReceipt() int {
	return m._callCountSlashInvalidReceipt
}

func (m *MockEffectsHappyPath) CallCountHandleError() int {
	return m._callCountHandleError
}
