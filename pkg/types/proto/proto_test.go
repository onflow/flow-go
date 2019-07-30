package proto_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/dapperlabs/bamboo-node/pkg/types/proto"
	"github.com/dapperlabs/bamboo-node/pkg/unittest"
)

func TestAccountSignature(t *testing.T) {
	RegisterTestingT(t)

	sigA := unittest.AccountSignatureFixture()

	message := proto.AccountSignatureToMessage(sigA)
	sigB := proto.MessageToAccountSignature(message)

	Expect(sigA).To(Equal(sigB))
}

func TestSignedTransaction(t *testing.T) {
	RegisterTestingT(t)

	txA := unittest.SignedTransactionFixture()

	message := proto.SignedTransactionToMessage(txA)
	txB := proto.MessageToSignedTransaction(message)

	Expect(txA).To(Equal(txB))
}
