package proto_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/dapperlabs/flow-go/pkg/types/proto"
	"github.com/dapperlabs/flow-go/pkg/utils/unittest"
)

func TestBlockHeader(t *testing.T) {
	RegisterTestingT(t)

	blockHeaderA := unittest.BlockHeaderFixture()

	message := proto.BlockHeaderToMessage(blockHeaderA)
	blockHeaderB := proto.MessageToBlockHeader(message)

	Expect(blockHeaderA).To(Equal(blockHeaderB))
}

func TestAccountSignature(t *testing.T) {
	RegisterTestingT(t)

	sigA := unittest.AccountSignatureFixture()

	message := proto.AccountSignatureToMessage(sigA)
	sigB := proto.MessageToAccountSignature(message)

	Expect(sigA).To(Equal(sigB))
}

func TestTransaction(t *testing.T) {
	RegisterTestingT(t)

	txA := unittest.TransactionFixture()

	message := proto.TransactionToMessage(txA)

	txB, err := proto.MessageToTransaction(message)
	Expect(err).ToNot(HaveOccurred())

	Expect(txA).To(Equal(txB))
}

func TestAccount(t *testing.T) {
	RegisterTestingT(t)

	accA := unittest.AccountFixture()

	message, err := proto.AccountToMessage(accA)
	Expect(err).ToNot(HaveOccurred())

	accB, err := proto.MessageToAccount(message)
	Expect(err).ToNot(HaveOccurred())

	Expect(accA).To(Equal(accB))
}

func TestAccountKey(t *testing.T) {
	RegisterTestingT(t)

	keyA := unittest.AccountKeyFixture()

	message, err := proto.AccountKeyToMessage(keyA)
	Expect(err).ToNot(HaveOccurred())

	keyB, err := proto.MessageToAccountKey(message)
	Expect(err).ToNot(HaveOccurred())

	Expect(keyA).To(Equal(keyB))
}
