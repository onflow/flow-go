package messageable

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestRegister(t *testing.T) {
	RegisterTestingT(t)

	message := mocks.MockRegisterMessage()
	register := mocks.MockRegister()

	actualMessage := RegisterToMessage(register)
	actualRegister := MessageToRegister(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualRegister).To(Equal(register))
}

func TestIntermediateRegisters(t *testing.T) {
	RegisterTestingT(t)

	message := mocks.MockIntermediateRegistersMessage()
	intermediateRegisters := mocks.MockIntermediateRegisters()

	actualMessage := IntermediateRegistersToMessage(intermediateRegisters)
	actualIntermediateRegisters := MessageToIntermediateRegisters(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualIntermediateRegisters).To(Equal(intermediateRegisters))
}

func TestTransactionRegister(t *testing.T) {
	RegisterTestingT(t)

	message := mocks.MockTransactionRegisterMessage()
	transactionRegister := mocks.MockTransactionRegister()

	actualMessage := TransactionRegisterToMessage(transactionRegister)
	actualTransactionRegister := MessageToTransactionRegister(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualTransactionRegister).To(Equal(transactionRegister))
}

func TestTransaction(t *testing.T) {
	RegisterTestingT(t)

	message := mocks.MockTransactionMessage()
	txn := mocks.MockTransaction()

	actualMessage := TransactionToMessage(txn)
	actualTxn := MessageToTransaction(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualTxn).To(Equal(txn))
}

func TestSignedTransaction(t *testing.T) {
	RegisterTestingT(t)

	message := mocks.MockSignedTransactionMessage()
	signedTxn := mocks.MockSignedTransaction()

	actualMessage := SignedTransactionToMessage(signedTxn)
	actualSignedTxn := MessageToSignedTransaction(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualSignedTxn).To(Equal(signedTxn))
}

func TestCollection(t *testing.T) {
	RegisterTestingT(t)

	message := mocks.MockCollectionMessage()
	collection := mocks.MockCollection()

	actualMessage := CollectionToMessage(collection)
	actualCollection := MessageToCollection(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualCollection).To(Equal(collection))
}

func TestSignedCollectionHash(t *testing.T) {
	RegisterTestingT(t)

	message := mocks.MockSignedCollectionHashMessage()
	signedCollectionHash := mocks.MockSignedCollectionHash()

	actualMessage := SignedCollectionHashToMessage(signedCollectionHash)
	actualSignedCollectionHash := MessageToSignedCollectionHash(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualSignedCollectionHash).To(Equal(signedCollectionHash))
}

func TestExecutionReceipt(t *testing.T) {
	RegisterTestingT(t)

	message := mocks.MockExecutionReceiptMessage()
	executionReceipt := mocks.MockExecutionReceipt()

	actualMessage := ExecutionReceiptToMessage(executionReceipt)
	actualExecutionReceipt := MessageToExecutionReceipt(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualExecutionReceipt).To(Equal(executionReceipt))
}

func TestInvalidExecutionReceiptChallenge(t *testing.T) {
	RegisterTestingT(t)

	message := mocks.MockInvalidExecutionReceiptChallengeMessage()
	invalidExecutionReceiptChallenge := mocks.MockInvalidExecutionReceiptChallenge()

	actualMessage := InvalidExecutionReceiptChallengeToMessage(invalidExecutionReceiptChallenge)
	actualIERC := MessageToInvalidExecutionReceiptChallenge(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualIERC).To(Equal(invalidExecutionReceiptChallenge))
}

func TestResultApproval(t *testing.T) {
	RegisterTestingT(t)

	message := mocks.MockResultApprovalMessage()
	resultApproval := mocks.MockResultApproval()

	actualMessage := ResultApprovalToMessage(resultApproval)
	actualResultApproval := MessageToResultApproval(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualResultApproval).To(Equal(resultApproval))
}

func TestBlockSeal(t *testing.T) {
	RegisterTestingT(t)

	message := mocks.MockBlockSealMessage()
	blockSeal := mocks.MockBlockSeal()

	actualMessage := BlockSealToMessage(blockSeal)
	actualBlockSeal := MessageToBlockSeal(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualBlockSeal).To(Equal(blockSeal))
}

func TestBlock(t *testing.T) {
	RegisterTestingT(t)

	message := mocks.MockBlockMessage()
	block := mocks.MockBlock()

	actualMessage := BlockToMessage(block)
	actualBlock := MessageToBlock(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualBlock).To(Equal(block))
}

func TestStateTransition(t *testing.T) {
	RegisterTestingT(t)

	message := mocks.MockStateTransitionMessage()
	stateTransition := mocks.MockStateTransition()

	actualMessage := StateTransitionToMessage(stateTransition)
	actualStateTransition := MessageToStateTransition(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualStateTransition).To(Equal(stateTransition))
}

func TestSignedStateTransition(t *testing.T) {
	RegisterTestingT(t)

	message := mocks.MockSignedStateTransitionMessage()
	signedStateTransition := mocks.MockSignedStateTransition()

	actualMessage := SignedStateTransitionToMessage(signedStateTransition)
	actualSignedStateTransition := MessageToSignedStateTransition(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualSignedStateTransition).To(Equal(signedStateTransition))
}

func TestFinalizedStateTransition(t *testing.T) {
	RegisterTestingT(t)

	message := mocks.MockFinalizedStateTransitionMessage()
	finalizedStateTransition := mocks.MockFinalizedStateTransition()

	actualMessage := FinalizedStateTransitionToMessage(finalizedStateTransition)
	actualFinalizedStateTransition := MessageToFinalizedStateTransition(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualFinalizedStateTransition).To(Equal(finalizedStateTransition))
}

func TestStateTransitionVote(t *testing.T) {
	RegisterTestingT(t)

	message := mocks.MockStateTransitionVoteMessage()
	stateTransitionVote := mocks.MockStateTransitionVote()

	actualMessage := StateTransitionVoteToMessage(stateTransitionVote)
	actualStateTransitionVote := MessageToStateTransitionVote(message)

	Expect(actualMessage).To(Equal(message))
	Expect(actualStateTransitionVote).To(Equal(stateTransitionVote))
}
