package proto_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/dapperlabs/bamboo-node/pkg/types/mocks"
	"github.com/dapperlabs/bamboo-node/pkg/types/proto"
)

func TestAccountSignature(t *testing.T) {
	RegisterTestingT(t)

	sigA := mocks.MockAccountSignature()

	message := proto.AccountSignatureToMessage(sigA)
	sigB := proto.MessageToAccountSignature(message)

	Expect(sigA).To(Equal(sigB))
}

func TestSignedTransaction(t *testing.T) {
	RegisterTestingT(t)

	txA := mocks.MockSignedTransaction()

	message, err := proto.SignedTransactionToMessage(txA)
	Expect(err).ToNot(HaveOccurred())

	txB, err := proto.MessageToSignedTransaction(message)
	Expect(err).ToNot(HaveOccurred())

	Expect(txA).To(Equal(txB))
}
