package processor

import (
	"fmt"
	"testing"
	"time"

	"github.com/dapperlabs/bamboo-node/internal/pkg/types"
	. "github.com/onsi/gomega"
)

func TestProcessorHappyPath(t *testing.T) {
	RegisterTestingT(t)

	m := &mockEffects{}
	c := &ReceiptProcessorConfig{
		QueueBuffer: 100,
		CacheBuffer: 100,
	}
	p := NewReceiptProcessor(m, c)

	receipt := &types.ExecutionReceipt{}
	done := make(chan bool, 0)
	p.Submit(receipt, done)

	select {
	case _ = <-done:
		Expect(m).To(Equal(&mockEffects{
			callCountIsValidExecutionReceipt:      1,
			callCountHasMinStake:                  1,
			callCountIsSealedWithDifferentReceipt: 1,
			callCountSend:                         1,
			callCountSlashExpiredReceipt:          0,
			callCountSlashInvalidReceipt:          0,
			callCountHandleError:                  0,
		}))
	case <-time.After(1 * time.Second):
		t.Error("waited for receipt to be processed for me than 1 sec")
	}

}

// mockEffects implements the processorEffects interface
type mockEffects struct {
	callCountIsValidExecutionReceipt      int
	callCountHasMinStake                  int
	callCountIsSealedWithDifferentReceipt int
	callCountSend                         int
	callCountSlashExpiredReceipt          int
	callCountSlashInvalidReceipt          int
	callCountHandleError                  int
}

func (m *mockEffects) isValidExecutionReceipt(*types.ExecutionReceipt) (ValidationResult, error) {
	m.callCountIsValidExecutionReceipt++
	return &ValidationResultSuccess{}, nil
}

func (m *mockEffects) hasMinStake(*types.ExecutionReceipt) (bool, error) {
	m.callCountHasMinStake++
	return true, nil
}

func (m *mockEffects) isSealedWithDifferentReceipt(*types.ExecutionReceipt) (bool, error) {
	m.callCountIsSealedWithDifferentReceipt++
	return false, nil
}

func (m *mockEffects) send(*types.ExecutionReceipt, []byte) error {
	m.callCountSend++
	return nil
}

func (m *mockEffects) slashExpiredReceipt(*types.ExecutionReceipt) error {
	m.callCountSlashExpiredReceipt++
	return nil
}

func (m *mockEffects) slashInvalidReceipt(*types.ExecutionReceipt, *types.BlockPartExecutionResult) error {
	m.callCountSlashInvalidReceipt++
	return nil
}

func (m *mockEffects) handleError(err error) {
	fmt.Println(err)
	m.callCountHandleError++
	return
}
