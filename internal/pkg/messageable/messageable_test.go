package messageable_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/dapperlabs/bamboo-node/internal/pkg/messageable"
	"github.com/dapperlabs/bamboo-node/internal/pkg/mocks"
)

func TestAccountSignature(t *testing.T) {
	RegisterTestingT(t)

	sigA := mocks.MockAccountSignature()

	message := messageable.AccountSignatureToMessage(sigA)
	sigB := messageable.MessageToAccountSignature(message)

	Expect(sigA).To(Equal(sigB))
}

func TestSignedTransaction(t *testing.T) {
	RegisterTestingT(t)

	txA := mocks.MockSignedTransaction()

	message := messageable.SignedTransactionToMessage(txA)
	txB := messageable.MessageToSignedTransaction(message)

	Expect(txA).To(Equal(txB))
}
