package proto_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/dapperlabs/bamboo-node/internal/pkg/mocks"
	"github.com/dapperlabs/bamboo-node/internal/pkg/proto"
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

	message := proto.SignedTransactionToMessage(txA)
	txB := proto.MessageToSignedTransaction(message)

	Expect(txA).To(Equal(txB))
}
