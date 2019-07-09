package utils

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/dapperlabs/bamboo-node/internal/types"
)

func TestRegister(t *testing.T) {
	gomega := NewWithT(t)

	message := types.MockRegisterMessage()
	register := types.MockRegister()

	actualMessage := RegisterToMessage(register)
	actualRegister := MessageToRegister(message)

	gomega.Expect(actualMessage).To(Equal(message))
	gomega.Expect(actualRegister).To(Equal(register))
	gomega.Expect(RegisterToMessage(actualRegister)).To(Equal(message))
	gomega.Expect(MessageToRegister(actualMessage)).To(Equal(register))
}

func TestIntermediateRegisters(t *testing.T) {
	gomega := NewWithT(t)

	message := types.MockIntermediateRegistersMessage()
	intermediateRegisters := types.MockIntermediateRegisters()

	actualMessage := IntermediateRegistersToMessage(intermediateRegisters)
	actualIntermediateRegisters := MessageToIntermediateRegisters(message)

	gomega.Expect(actualMessage).To(Equal(message))
	gomega.Expect(actualIntermediateRegisters).To(Equal(intermediateRegisters))
	gomega.Expect(IntermediateRegistersToMessage(actualIntermediateRegisters)).To(Equal(message))
	gomega.Expect(MessageToIntermediateRegisters(actualMessage)).To(Equal(intermediateRegisters))
}

func TestTransactionRegister(t *testing.T) {
	gomega := NewWithT(t)

	message := types.MockTransactionRegisterMessage()
	transactionRegister := types.MockTransactionRegister()

	actualMessage := TransactionRegisterToMessage(transactionRegister)
	actualTransactionRegister := MessageToTransactionRegister(message)

	gomega.Expect(actualMessage).To(Equal(message))
	gomega.Expect(actualTransactionRegister).To(Equal(transactionRegister))
	gomega.Expect(TransactionRegisterToMessage(actualTransactionRegister)).To(Equal(message))
	gomega.Expect(MessageToTransactionRegister(actualMessage)).To(Equal(transactionRegister))
}

func TestTransaction(t *testing.T) {
	gomega := NewWithT(t)

	message := types.MockTransactionMessage()
	txn := types.MockTransaction()

	actualMessage := TransactionToMessage(txn)
	actualTxn := MessageToTransaction(message)

	gomega.Expect(actualMessage).To(Equal(message))
	gomega.Expect(actualTxn).To(Equal(txn))
	gomega.Expect(TransactionToMessage(actualTxn)).To(Equal(message))
	gomega.Expect(MessageToTransaction(actualMessage)).To(Equal(txn))
}

func TestSignedTransaction(t *testing.T) {
	gomega := NewWithT(t)

	message := types.MockSignedTransactionMessage()
	signedTxn := types.MockSignedTransaction()

	actualMessage := SignedTransactionToMessage(signedTxn)
	actualSignedTxn := MessageToSignedTransaction(message)

	gomega.Expect(actualMessage).To(Equal(message))
	gomega.Expect(actualSignedTxn).To(Equal(signedTxn))
	gomega.Expect(SignedTransactionToMessage(actualSignedTxn)).To(Equal(message))
	gomega.Expect(MessageToSignedTransaction(actualMessage)).To(Equal(signedTxn))
}

func TestCollection(t *testing.T) {
	gomega := NewWithT(t)

	message := types.MockCollectionMessage()
	collection := types.MockCollection()

	actualMessage := CollectionToMessage(collection)
	actualCollection := MessageToCollection(message)

	gomega.Expect(actualMessage).To(Equal(message))
	gomega.Expect(actualCollection).To(Equal(collection))
	gomega.Expect(CollectionToMessage(actualCollection)).To(Equal(message))
	gomega.Expect(MessageToCollection(actualMessage)).To(Equal(collection))
}

func TestSignedCollectionHash(t *testing.T) {
	gomega := NewWithT(t)

	message := types.MockSignedCollectionHashMessage()
	signedCollectionHash := types.MockSignedCollectionHash()

	actualMessage := SignedCollectionHashToMessage(signedCollectionHash)
	actualSignedCollectionHash := MessageToSignedCollectionHash(message)

	gomega.Expect(actualMessage).To(Equal(message))
	gomega.Expect(actualSignedCollectionHash).To(Equal(signedCollectionHash))
	gomega.Expect(SignedCollectionHashToMessage(actualSignedCollectionHash)).To(Equal(message))
	gomega.Expect(MessageToSignedCollectionHash(actualMessage)).To(Equal(signedCollectionHash))
}
