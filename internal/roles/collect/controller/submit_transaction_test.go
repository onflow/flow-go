package controller_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	"github.com/dapperlabs/bamboo-node/internal/roles/collect/controller"
	"github.com/dapperlabs/bamboo-node/internal/roles/collect/storage"
	"github.com/dapperlabs/bamboo-node/internal/roles/collect/txpool"
	svc "github.com/dapperlabs/bamboo-node/pkg/grpc/services/collect"
	"github.com/dapperlabs/bamboo-node/pkg/types"
	"github.com/dapperlabs/bamboo-node/pkg/types/proto"
	"github.com/dapperlabs/bamboo-node/pkg/utils/unittest"
)

type transactionTestCase struct {
	title         string
	tx            types.SignedTransaction
	shouldSucceed bool
}

var transactionTests = []transactionTestCase{
	{
		title:         "valid transaction",
		tx:            unittest.SignedTransactionFixture(),
		shouldSucceed: true,
	},
	{
		title: "transaction with no script should be rejected",
		tx: types.SignedTransaction{
			Nonce:          10,
			ComputeLimit:   5,
			PayerSignature: unittest.AccountSignatureFixture(),
		},
		shouldSucceed: false,
	},
	{
		title: "transaction with no compute limit should be rejected",
		tx: types.SignedTransaction{
			Nonce:          10,
			Script:         []byte("fun main() {}"),
			PayerSignature: unittest.AccountSignatureFixture(),
		},
		shouldSucceed: false,
	},
}

func TestSubmitTransaction(t *testing.T) {
	for _, tt := range transactionTests {
		t.Run(tt.title, func(t *testing.T) {
			RegisterTestingT(t)

			c := controller.New(storage.NewMockStorage(), txpool.New(), logrus.New())

			txMsg, err := proto.SignedTransactionToMessage(tt.tx)
			Expect(err).ToNot(HaveOccurred())

			_, err = c.SubmitTransaction(
				context.Background(),
				&svc.SubmitTransactionRequest{Transaction: txMsg},
			)

			if tt.shouldSucceed {
				Expect(err).ToNot(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
			}
		})
	}
}

func TestSubmitTransactionStorage(t *testing.T) {
	RegisterTestingT(t)

	// expose transaction pool and storage for use in tests
	txPool := txpool.New()
	store := storage.NewMockStorage()

	c := controller.New(store, txPool, logrus.New())

	txA := unittest.SignedTransactionFixture()
	txMsg, err := proto.SignedTransactionToMessage(txA)
	Expect(err).ToNot(HaveOccurred())

	// submit transaction
	_, err = c.SubmitTransaction(
		context.Background(),
		&svc.SubmitTransactionRequest{Transaction: txMsg},
	)
	Expect(err).ToNot(HaveOccurred())

	// stored transaction should be identical to submitted transaction
	txB, err := store.GetTransaction(txA.Hash())
	Expect(err).ToNot(HaveOccurred())
	Expect(txA).To(Equal(txB))

	// pending transaction should be identical to submitted transaction
	txC := txPool.Get(txA.Hash())
	Expect(txA).To(Equal(txC))
}

func TestSubmitDuplicateTransaction(t *testing.T) {
	RegisterTestingT(t)

	// expose transaction pool for use in tests
	txPool := txpool.New()

	c := controller.New(storage.NewMockStorage(), txPool, logrus.New())

	txA := unittest.SignedTransactionFixture()
	txMsg, err := proto.SignedTransactionToMessage(txA)
	Expect(err).ToNot(HaveOccurred())

	// submit transaction
	_, err = c.SubmitTransaction(
		context.Background(),
		&svc.SubmitTransactionRequest{Transaction: txMsg},
	)
	Expect(err).ToNot(HaveOccurred())

	// manually remove the transaction from the pending pool
	txPool.Remove(txA.Hash())

	// resubmit transaction
	_, err = c.SubmitTransaction(
		context.Background(),
		&svc.SubmitTransactionRequest{Transaction: txMsg},
	)
	Expect(err).ToNot(HaveOccurred())

	// transaction should not be added back to pool on 2nd submission
	containsTx := txPool.Contains(txA.Hash())
	Expect(containsTx).To(BeFalse())
}
