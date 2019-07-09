package types

import (
	bambooProto "github.com/dapperlabs/bamboo-node/grpc/shared"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

func MockHash() crypto.Hash {
	return crypto.Hash{}
}

func MockSignature() crypto.Signature {
	return crypto.Signature{}
}

func MockKeyWeight() KeyWeight {
	return KeyWeight{
		Key:    []byte("MOCK"),
		Weight: 0,
	}
}

func MockKeyWeightMessage() *bambooProto.TransactionRegister_KeyWeight {
	return &bambooProto.TransactionRegister_KeyWeight{
		Key:    []byte("MOCK"),
		Weight: 0,
	}
}

func MockRegister() *Register {
	return &Register{
		ID:    []byte("TEST"),
		Value: []byte("MOCK"),
	}
}

func MockRegisterMessage() *bambooProto.Register {
	return &bambooProto.Register{
		Id:    []byte("TEST"),
		Value: []byte("MOCK"),
	}
}

func MockIntermediateRegisters() *IntermediateRegisters {
	registers := make([]Register, 0)
	for i := 0; i < 5; i++ {
		registers = append(registers, *MockRegister())
	}

	return &IntermediateRegisters{
		TransactionHash: MockHash(),
		Registers:       registers,
		ComputeUsed:     0,
	}
}

func MockIntermediateRegistersMessage() *bambooProto.IntermediateRegisters {
	registers := make([]*bambooProto.Register, 0)
	for i := 0; i < 5; i++ {
		registers = append(registers, MockRegisterMessage())
	}

	return &bambooProto.IntermediateRegisters{
		TransactionHash: MockHash().Bytes(),
		Registers:       registers,
		ComputeUsed:     0,
	}
}

func MockTransactionRegister() *TransactionRegister {
	keys := make([]KeyWeight, 0)
	for i := 0; i < 5; i++ {
		keys = append(keys, MockKeyWeight())
	}

	return &TransactionRegister{
		Type:       SIMPLE,
		AccessMode: CREATE,
		ID:         []byte("TEST"),
		Keys:       keys,
	}
}

func MockTransactionRegisterMessage() *bambooProto.TransactionRegister {
	keys := make([]*bambooProto.TransactionRegister_KeyWeight, 0)
	for i := 0; i < 5; i++ {
		keys = append(keys, MockKeyWeightMessage())
	}

	return &bambooProto.TransactionRegister{
		Type:       bambooProto.TransactionRegister_SIMPLE,
		AccessMode: bambooProto.TransactionRegister_CREATE,
		Id:         []byte("TEST"),
		Keys:       keys,
	}
}

func MockTransaction() *Transaction {
	registers := make([]TransactionRegister, 0)
	for i := 0; i < 5; i++ {
		registers = append(registers, *MockTransactionRegister())
	}

	return &Transaction{
		Script:    []byte("TEST"),
		Nonce:     0,
		Registers: registers,
		Chunks:    make([][]byte, 0),
	}
}

func MockTransactionMessage() *bambooProto.Transaction {
	registers := make([]*bambooProto.TransactionRegister, 0)
	for i := 0; i < 5; i++ {
		registers = append(registers, MockTransactionRegisterMessage())
	}

	return &bambooProto.Transaction{
		Script:    []byte("TEST"),
		Nonce:     0,
		Registers: registers,
		Chunks:    make([][]byte, 0),
	}
}

func MockSignedTransaction() *SignedTransaction {
	sigs := make([]crypto.Signature, 0)
	for i := 0; i < 5; i++ {
		sigs = append(sigs, MockSignature())
	}

	return &SignedTransaction{
		Transaction:      *MockTransaction(),
		ScriptSignatures: sigs,
		PayerSignature:   MockSignature(),
	}
}

func MockSignedTransactionMessage() *bambooProto.SignedTransaction {
	sigs := make([][]byte, 0)
	for i := 0; i < 5; i++ {
		sigs = append(sigs, MockSignature().Bytes())
	}

	return &bambooProto.SignedTransaction{
		Transaction:      MockTransactionMessage(),
		ScriptSignatures: sigs,
		PayerSignature:   MockSignature().Bytes(),
	}
}

func MockCollection() *Collection {
	transactions := make([]SignedTransaction, 0)
	for i := 0; i < 5; i++ {
		transactions = append(transactions, *MockSignedTransaction())
	}

	return &Collection{
		Transactions:        transactions,
		FoundationBlockHash: MockHash(),
	}
}

func MockCollectionMessage() *bambooProto.Collection {
	transactions := make([]*bambooProto.SignedTransaction, 0)
	for i := 0; i < 5; i++ {
		transactions = append(transactions, MockSignedTransactionMessage())
	}

	return &bambooProto.Collection{
		Transactions:        transactions,
		FoundationBlockHash: MockHash().Bytes(),
	}
}

func MockSignedCollectionHash()  *SignedCollectionHash {
	sigs := make([]crypto.Signature, 0)
	for i := 0; i < 5; i++ {
		sigs = append(sigs, MockSignature())
	}
	
	return &SignedCollectionHash{
		CollectionHash:	MockHash(),
		Signatures:	sigs,
	}
}

func MockSignedCollectionHashMessage()  *bambooProto.SignedCollectionHash {
	sigs := make([][]byte, 0)
	for i := 0; i < 5; i++ {
		sigs = append(sigs, MockSignature().Bytes())
	}

	return &bambooProto.SignedCollectionHash{
		CollectionHash:	MockHash().Bytes(),
		Signatures:	sigs,
	}
}
