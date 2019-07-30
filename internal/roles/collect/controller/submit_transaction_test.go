package controller_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	svc "github.com/dapperlabs/bamboo-node/pkg/grpc/services/collect"

	"github.com/dapperlabs/bamboo-node/internal/roles/collect/controller"
	"github.com/dapperlabs/bamboo-node/pkg/types"
	"github.com/dapperlabs/bamboo-node/pkg/types/proto"
	"github.com/dapperlabs/bamboo-node/pkg/utils/unittest"
)

func TestSubmitTransaction(t *testing.T) {
	RegisterTestingT(t)

	c := controller.New(logrus.New())

	ctx := context.Background()

	tx := unittest.SignedTransactionFixture()

	txMsg, err := proto.SignedTransactionToMessage(tx)
	Expect(err).ToNot(HaveOccurred())

	_, err = c.SubmitTransaction(ctx, &svc.SubmitTransactionRequest{
		Transaction: txMsg,
	})
	Expect(err).ToNot(HaveOccurred())
}

func TestSubmitInvalidTransaction(t *testing.T) {
	RegisterTestingT(t)

	c := controller.New(logrus.New())

	ctx := context.Background()

	// transaction missing script and compute_limit fields
	tx := types.SignedTransaction{
		Nonce:          10,
		PayerSignature: unittest.AccountSignatureFixture(),
	}

	txMsg, err := proto.SignedTransactionToMessage(tx)
	Expect(err).ToNot(HaveOccurred())

	_, err = c.SubmitTransaction(ctx, &svc.SubmitTransactionRequest{
		Transaction: txMsg,
	})
	Expect(err).To(HaveOccurred())
}
