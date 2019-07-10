package utils

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/dapperlabs/bamboo-node/internal/types"
)

func TestRegister(t *testing.T) {
	RegisterTestingT(t)

	message := types.MockRegisterMessage()
	register := types.MockRegister()

	actualMessage := RegisterToMessage(register)
	actualRegister := MessageToRegister(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualRegister).To(Equal(register))
}

func TestIntermediateRegisters(t *testing.T) {
	RegisterTestingT(t)

	message := types.MockIntermediateRegistersMessage()
	intermediateRegisters := types.MockIntermediateRegisters()

	actualMessage := IntermediateRegistersToMessage(intermediateRegisters)
	actualIntermediateRegisters := MessageToIntermediateRegisters(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualIntermediateRegisters).To(Equal(intermediateRegisters))
}

func TestTransactionRegister(t *testing.T) {
	RegisterTestingT(t)

	message := types.MockTransactionRegisterMessage()
	transactionRegister := types.MockTransactionRegister()

	actualMessage := TransactionRegisterToMessage(transactionRegister)
	actualTransactionRegister := MessageToTransactionRegister(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualTransactionRegister).To(Equal(transactionRegister))
}

func TestTransaction(t *testing.T) {
	RegisterTestingT(t)

	message := types.MockTransactionMessage()
	txn := types.MockTransaction()

	actualMessage := TransactionToMessage(txn)
	actualTxn := MessageToTransaction(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualTxn).To(Equal(txn))
}

func TestSignedTransaction(t *testing.T) {
	RegisterTestingT(t)

	message := types.MockSignedTransactionMessage()
	signedTxn := types.MockSignedTransaction()

	actualMessage := SignedTransactionToMessage(signedTxn)
	actualSignedTxn := MessageToSignedTransaction(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualSignedTxn).To(Equal(signedTxn))
}

func TestCollection(t *testing.T) {
	RegisterTestingT(t)

	message := types.MockCollectionMessage()
	collection := types.MockCollection()

	actualMessage := CollectionToMessage(collection)
	actualCollection := MessageToCollection(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualCollection).To(Equal(collection))
}

func TestSignedCollectionHash(t *testing.T) {
	RegisterTestingT(t)

	message := types.MockSignedCollectionHashMessage()
	signedCollectionHash := types.MockSignedCollectionHash()

	actualMessage := SignedCollectionHashToMessage(signedCollectionHash)
	actualSignedCollectionHash := MessageToSignedCollectionHash(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualSignedCollectionHash).To(Equal(signedCollectionHash))
}

func TestExecutionReceipt(t *testing.T) {
	RegisterTestingT(t)

	message := types.MockExecutionReceiptMessage()
	executionReceipt := types.MockExecutionReceipt()

	actualMessage := ExecutionReceiptToMessage(executionReceipt)
	actualExecutionReceipt := MessageToExecutionReceipt(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualExecutionReceipt).To(Equal(executionReceipt))
}

func TestInvalidExecutionReceiptChallenge(t *testing.T) {
	RegisterTestingT(t)

	message := types.MockInvalidExecutionReceiptChallengeMessage()
	invalidExecutionReceiptChallenge := types.MockInvalidExecutionReceiptChallenge()

	actualMessage := InvalidExecutionReceiptChallengeToMessage(invalidExecutionReceiptChallenge)
	actualIERC := MessageToInvalidExecutionReceiptChallenge(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualIERC).To(Equal(invalidExecutionReceiptChallenge))
}

func TestResultApproval(t *testing.T) {
	RegisterTestingT(t)

	message := types.MockResultApprovalMessage()
	resultApproval := types.MockResultApproval()

	actualMessage := ResultApprovalToMessage(resultApproval)
	actualResultApproval := MessageToResultApproval(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualResultApproval).To(Equal(resultApproval))
}

func TestBlockSeal(t *testing.T) {
	RegisterTestingT(t)

	message := types.MockBlockSealMessage()
	blockSeal := types.MockBlockSeal()

	actualMessage := BlockSealToMessage(blockSeal)
	actualBlockSeal := MessageToBlockSeal(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualBlockSeal).To(Equal(blockSeal))
}

func TestBlock(t *testing.T) {
	RegisterTestingT(t)

	message := types.MockBlockMessage()
	block := types.MockBlock()

	actualMessage := BlockToMessage(block)
	actualBlock := MessageToBlock(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualBlock).To(Equal(block))
}

func TestStateTransition(t *testing.T) {
	RegisterTestingT(t)

	message := types.MockStateTransitionMessage()
	stateTransition := types.MockStateTransition()

	actualMessage := StateTransitionToMessage(stateTransition)
	actualStateTransition := MessageToStateTransition(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualStateTransition).To(Equal(stateTransition))
}

func TestSignedStateTransition(t *testing.T) {
	RegisterTestingT(t)

	message := types.MockSignedStateTransitionMessage()
	signedStateTransition := types.MockSignedStateTransition()

	actualMessage := SignedStateTransitionToMessage(signedStateTransition)
	actualSignedStateTransition := MessageToSignedStateTransition(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualSignedStateTransition).To(Equal(signedStateTransition))
}

func TestFinalizedStateTransition(t *testing.T) {
	RegisterTestingT(t)

	message := types.MockFinalizedStateTransitionMessage()
	finalizedStateTransition := types.MockFinalizedStateTransition()

	actualMessage := FinalizedStateTransitionToMessage(finalizedStateTransition)
	actualFinalizedStateTransition := MessageToFinalizedStateTransition(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualFinalizedStateTransition).To(Equal(finalizedStateTransition))
}
