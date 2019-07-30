package controller_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	svc "github.com/dapperlabs/bamboo-node/pkg/grpc/services/collect"

	"github.com/dapperlabs/bamboo-node/internal/roles/collect/controller"
	"github.com/dapperlabs/bamboo-node/internal/roles/collect/data"
	"github.com/dapperlabs/bamboo-node/internal/roles/collect/txpool"
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

			c := controller.New(data.NewMock(), txpool.New(), logrus.New())

			txMsg, err := proto.SignedTransactionToMessage(tt.tx)
			Expect(err).ToNot(HaveOccurred())

			_, err = c.SubmitTransaction(
				context.Background(),
				&svc.SubmitTransactionRequest{
					Transaction: txMsg,
				},
			)

			if tt.shouldSucceed {
				Expect(err).ToNot(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
			}
		})
	}
}
