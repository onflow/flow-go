package processor

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/dapperlabs/bamboo-node/internal/pkg/types"
	. "github.com/dapperlabs/bamboo-node/internal/roles/verify/processor/test_mocks"
)

func TestHappyPath(t *testing.T) {
	RegisterTestingT(t)

	m := &MockEffectsHappyPath{}
	c := &ReceiptProcessorConfig{
		QueueBuffer: 100,
		CacheBuffer: 100,
	}
	p := NewReceiptProcessor(m, c)

	receipt := &types.ExecutionReceipt{}
	done := make(chan bool, 1)
	p.Submit(receipt, done)

	select {
	case _ = <-done:
		Expect(m.CallCountIsValidExecutionReceipt()).To(Equal(1))
		Expect(m.CallCountHasMinStake()).To(Equal(1))
		Expect(m.CallCountIsSealedWithDifferentReceipt()).To(Equal(1))
		Expect(m.CallCountSend()).To(Equal(1))
		Expect(m.CallCountSlashExpiredReceipt()).To(Equal(0))
		Expect(m.CallCountSlashInvalidReceipt()).To(Equal(0))
		Expect(m.CallCountHandleError()).To(Equal(0))
	case <-time.After(1 * time.Second):
		t.Error("waited for receipt to be processed for me than 1 sec")
	}

}

func TestNoMinStake(t *testing.T) {

	RegisterTestingT(t)

	m := NewMockEffectsNoMinStake(&MockEffectsHappyPath{})
	c := &ReceiptProcessorConfig{
		QueueBuffer: 100,
		CacheBuffer: 100,
	}
	p := NewReceiptProcessor(m, c)

	receipt := &types.ExecutionReceipt{}
	done := make(chan bool, 1)
	p.Submit(receipt, done)

	select {
	case _ = <-done:
		Expect(m.CallCountIsValidExecutionReceipt()).To(Equal(0))
		Expect(m.CallCountHasMinStake()).To(Equal(1))
		Expect(m.CallCountIsSealedWithDifferentReceipt()).To(Equal(0))
		Expect(m.CallCountSend()).To(Equal(0))
		Expect(m.CallCountSlashExpiredReceipt()).To(Equal(0))
		Expect(m.CallCountSlashInvalidReceipt()).To(Equal(0))
		Expect(m.CallCountHandleError()).To(Equal(1))
	case <-time.After(1 * time.Second):
		t.Error("waited for receipt to be processed for me than 1 sec")
	}

}
