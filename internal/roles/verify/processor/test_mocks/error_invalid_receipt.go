package testmocks

import (
	"errors"

	"github.com/dapperlabs/flow-go/internal/roles/verify/compute"
	"github.com/dapperlabs/flow-go/pkg/types"
)

func NewMockEffectsErrorReceiptValidation(m *MockEffectsHappyPath) *MockEffectsErrorReceiptValidation {
	return &MockEffectsErrorReceiptValidation{m, 0}
}

// MockEffectsErrorReceiptValidation implements the processor.Effects & Mock interfaces
type MockEffectsErrorReceiptValidation struct {
	*MockEffectsHappyPath
	_callCount int
}

func (m *MockEffectsErrorReceiptValidation) IsValidExecutionReceipt(*types.ExecutionReceipt) (compute.ValidationResult, error) {
	m._callCount++
	return nil, errors.New("Validation Error")
}

func (m *MockEffectsErrorReceiptValidation) CallCountIsValidExecutionReceipt() int {
	return m._callCount
}
