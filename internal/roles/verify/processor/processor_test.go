package processor_test

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/dapperlabs/bamboo-node/internal/roles/verify/processor"
	. "github.com/dapperlabs/bamboo-node/internal/roles/verify/processor/test_mocks"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

type test struct {
	title      string
	m          Mock
	expectFunc func(Mock, *testing.T)
}

func Test(t *testing.T) {

	tests := []test{
		test{
			title: "Happy Path",
			m:     &MockEffectsHappyPath{},
			expectFunc: func(m Mock, t *testing.T) {
				RegisterTestingT(t)
				Expect(m.CallCountIsValidExecutionReceipt()).To(Equal(1))
				Expect(m.CallCountHasMinStake()).To(Equal(1))
				Expect(m.CallCountIsSealedWithDifferentReceipt()).To(Equal(1))
				Expect(m.CallCountSend()).To(Equal(1))
				Expect(m.CallCountSlashExpiredReceipt()).To(Equal(0))
				Expect(m.CallCountSlashInvalidReceipt()).To(Equal(0))
				Expect(m.CallCountHandleError()).To(Equal(0))
			},
		},
		test{
			title: "It should discard if receipt does not meet min stake",
			m:     NewMockEffectsNoMinStake(&MockEffectsHappyPath{}),
			expectFunc: func(m Mock, t *testing.T) {
				RegisterTestingT(t)
				Expect(m.CallCountIsValidExecutionReceipt()).To(Equal(0))
				Expect(m.CallCountHasMinStake()).To(Equal(1))
				Expect(m.CallCountIsSealedWithDifferentReceipt()).To(Equal(0))
				Expect(m.CallCountSend()).To(Equal(0))
				Expect(m.CallCountSlashExpiredReceipt()).To(Equal(0))
				Expect(m.CallCountSlashInvalidReceipt()).To(Equal(0))
				Expect(m.CallCountHandleError()).To(Equal(1))
			},
		},
		test{
			title: "It should discard an errored call of min stake",
			m:     NewMockEffectsErrorMinStake(&MockEffectsHappyPath{}),
			expectFunc: func(m Mock, t *testing.T) {
				RegisterTestingT(t)
				Expect(m.CallCountIsValidExecutionReceipt()).To(Equal(0))
				Expect(m.CallCountHasMinStake()).To(Equal(1))
				Expect(m.CallCountIsSealedWithDifferentReceipt()).To(Equal(0))
				Expect(m.CallCountSend()).To(Equal(0))
				Expect(m.CallCountSlashExpiredReceipt()).To(Equal(0))
				Expect(m.CallCountSlashInvalidReceipt()).To(Equal(0))
				Expect(m.CallCountHandleError()).To(Equal(1))
			},
		},
		test{
			title: "It should slash if sealed with different receipt",
			m:     NewMockEffectsSealWithDifferentReceipt(&MockEffectsHappyPath{}),
			expectFunc: func(m Mock, t *testing.T) {
				RegisterTestingT(t)
				Expect(m.CallCountIsValidExecutionReceipt()).To(Equal(0))
				Expect(m.CallCountHasMinStake()).To(Equal(1))
				Expect(m.CallCountIsSealedWithDifferentReceipt()).To(Equal(1))
				Expect(m.CallCountSend()).To(Equal(0))
				Expect(m.CallCountSlashExpiredReceipt()).To(Equal(1))
				Expect(m.CallCountSlashInvalidReceipt()).To(Equal(0))
				Expect(m.CallCountHandleError()).To(Equal(0))
			},
		},
		test{
			title: "It should discard if errored call of sealed with different receipt",
			m:     NewMockEffectsErrorSealWithDifferentReceipt(&MockEffectsHappyPath{}),
			expectFunc: func(m Mock, t *testing.T) {
				RegisterTestingT(t)
				Expect(m.CallCountIsValidExecutionReceipt()).To(Equal(0))
				Expect(m.CallCountHasMinStake()).To(Equal(1))
				Expect(m.CallCountIsSealedWithDifferentReceipt()).To(Equal(1))
				Expect(m.CallCountSend()).To(Equal(0))
				Expect(m.CallCountSlashExpiredReceipt()).To(Equal(0))
				Expect(m.CallCountSlashInvalidReceipt()).To(Equal(0))
				Expect(m.CallCountHandleError()).To(Equal(1))
			},
		},
		test{
			title: "It should slash if invalid receipt",
			m:     NewMockEffectsInvalidReceipt(&MockEffectsHappyPath{}),
			expectFunc: func(m Mock, t *testing.T) {
				RegisterTestingT(t)
				Expect(m.CallCountIsValidExecutionReceipt()).To(Equal(1))
				Expect(m.CallCountHasMinStake()).To(Equal(1))
				Expect(m.CallCountIsSealedWithDifferentReceipt()).To(Equal(1))
				Expect(m.CallCountSend()).To(Equal(0))
				Expect(m.CallCountSlashExpiredReceipt()).To(Equal(0))
				Expect(m.CallCountSlashInvalidReceipt()).To(Equal(1))
				Expect(m.CallCountHandleError()).To(Equal(0))
			},
		},
		test{
			title: "It should discard an errored call of validation",
			m:     NewMockEffectsErrorReceiptValidation(&MockEffectsHappyPath{}),
			expectFunc: func(m Mock, t *testing.T) {
				RegisterTestingT(t)
				Expect(m.CallCountIsValidExecutionReceipt()).To(Equal(1))
				Expect(m.CallCountHasMinStake()).To(Equal(1))
				Expect(m.CallCountIsSealedWithDifferentReceipt()).To(Equal(1))
				Expect(m.CallCountSend()).To(Equal(0))
				Expect(m.CallCountSlashExpiredReceipt()).To(Equal(0))
				Expect(m.CallCountSlashInvalidReceipt()).To(Equal(0))
				Expect(m.CallCountHandleError()).To(Equal(1))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.title, func(t *testing.T) {
			RegisterTestingT(t)

			c := &processor.ReceiptProcessorConfig{
				QueueBuffer: 100,
				CacheBuffer: 100,
			}
			hasher := crypto.NewHashAlgo(crypto.SHA3_256)
			p := processor.NewReceiptProcessor(test.m, c, hasher)

			receipt := &processor.ExecutionReceipt{}
			done := make(chan bool, 1)
			p.Submit(receipt, done)

			select {
			case _ = <-done:
				test.expectFunc(test.m, t)
			case <-time.After(1 * time.Second):
				t.Error("waited for receipt to be processed for me than 1 sec")
			}

		})
	}
}
