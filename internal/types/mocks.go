package types

import (
	"time"

	"github.com/golang/protobuf/ptypes"

	bambooProto "github.com/dapperlabs/bamboo-node/grpc/shared"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

var currentTime = time.Date(2009, 11, 17, 20, 34, 58, 651387237, time.UTC)

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

func MockSignedCollectionHash() *SignedCollectionHash {
	sigs := make([]crypto.Signature, 0)
	for i := 0; i < 5; i++ {
		sigs = append(sigs, MockSignature())
	}

	return &SignedCollectionHash{
		CollectionHash: MockHash(),
		Signatures:     sigs,
	}
}

func MockSignedCollectionHashMessage() *bambooProto.SignedCollectionHash {
	sigs := make([][]byte, 0)
	for i := 0; i < 5; i++ {
		sigs = append(sigs, MockSignature().Bytes())
	}

	return &bambooProto.SignedCollectionHash{
		CollectionHash: MockHash().Bytes(),
		Signatures:     sigs,
	}
}

func MockExecutionReceipt() *ExecutionReceipt {
	registers := make([]Register, 0)
	irList := make([]IntermediateRegisters, 0)
	sigs := make([]crypto.Signature, 0)
	for i := 0; i < 5; i++ {
		registers = append(registers, *MockRegister())
		irList = append(irList, *MockIntermediateRegisters())
		sigs = append(sigs, MockSignature())
	}

	return &ExecutionReceipt{
		PreviousReceiptHash:       MockHash(),
		BlockHash:                 MockHash(),
		InitialRegisters:          registers,
		IntermediateRegistersList: irList,
		Signatures:                sigs,
	}
}

func MockExecutionReceiptMessage() *bambooProto.ExecutionReceipt {
	registers := make([]*bambooProto.Register, 0)
	irList := make([]*bambooProto.IntermediateRegisters, 0)
	sigs := make([][]byte, 0)
	for i := 0; i < 5; i++ {
		registers = append(registers, MockRegisterMessage())
		irList = append(irList, MockIntermediateRegistersMessage())
		sigs = append(sigs, MockSignature().Bytes())
	}

	return &bambooProto.ExecutionReceipt{
		PreviousReceiptHash:       MockHash().Bytes(),
		BlockHash:                 MockHash().Bytes(),
		InitialRegisters:          registers,
		IntermediateRegistersList: irList,
		Signatures:                sigs,
	}
}

func MockInvalidExecutionReceiptChallenge() *InvalidExecutionReceiptChallenge {
	partTransactions := make([]IntermediateRegisters, 0)
	for i := 0; i < 5; i++ {
		partTransactions = append(partTransactions, *MockIntermediateRegisters())
	}

	return &InvalidExecutionReceiptChallenge{
		ExecutionReceiptHash:      MockHash(),
		ExecutionReceiptSignature: MockSignature(),
		PartIndex:                 0,
		PartTransactions:          partTransactions,
		Signature:                 MockSignature(),
	}
}

func MockInvalidExecutionReceiptChallengeMessage() *bambooProto.InvalidExecutionReceiptChallenge {
	partTransactions := make([]*bambooProto.IntermediateRegisters, 0)
	for i := 0; i < 5; i++ {
		partTransactions = append(partTransactions, MockIntermediateRegistersMessage())
	}

	return &bambooProto.InvalidExecutionReceiptChallenge{
		ExecutionReceiptHash:      MockHash().Bytes(),
		ExecutionReceiptSignature: MockSignature().Bytes(),
		PartIndex:                 0,
		PartTransactions:          partTransactions,
		Signature:                 MockSignature().Bytes(),
	}
}

func MockResultApproval() *ResultApproval {
	return &ResultApproval{
		BlockHeight:             0,
		ExecutionReceiptHash:    MockHash(),
		ResultApprovalSignature: MockSignature(),
		Proof:                   0,
		Signature:               MockSignature(),
	}
}

func MockResultApprovalMessage() *bambooProto.ResultApproval {
	return &bambooProto.ResultApproval{
		BlockHeight:             0,
		ExecutionReceiptHash:    MockHash().Bytes(),
		ResultApprovalSignature: MockSignature().Bytes(),
		Proof:                   0,
		Signature:               MockSignature().Bytes(),
	}
}

func MockBlockSeal() *BlockSeal {
	erSigs := make([]crypto.Signature, 0)
	raSigs := make([]crypto.Signature, 0)
	for i := 0; i < 5; i++ {
		erSigs = append(erSigs, MockSignature())
		raSigs = append(raSigs, MockSignature())
	}

	return &BlockSeal{
		BlockHash:                  MockHash(),
		ExecutionReceiptHash:       MockHash(),
		ExecutionReceiptSignatures: erSigs,
		ResultApprovalSignatures:   raSigs,
	}
}

func MockBlockSealMessage() *bambooProto.BlockSeal {
	erSigs := make([][]byte, 0)
	raSigs := make([][]byte, 0)
	for i := 0; i < 5; i++ {
		erSigs = append(erSigs, MockSignature().Bytes())
		raSigs = append(raSigs, MockSignature().Bytes())
	}

	return &bambooProto.BlockSeal{
		BlockHash:                  MockHash().Bytes(),
		ExecutionReceiptHash:       MockHash().Bytes(),
		ExecutionReceiptSignatures: erSigs,
		ResultApprovalSignatures:   raSigs,
	}
}

func MockBlock() *Block {
	signedCollectionHashes := make([]SignedCollectionHash, 0)
	blockSeals := make([]BlockSeal, 0)
	sigs := make([]crypto.Signature, 0)
	for i := 0; i < 5; i++ {
		signedCollectionHashes = append(signedCollectionHashes, *MockSignedCollectionHash())
		blockSeals = append(blockSeals, *MockBlockSeal())
		sigs = append(sigs, MockSignature())
	}

	return &Block{
		ChainID:                "BAMBOO",
		Height:                 0,
		PreviousBlockHash:      MockHash(),
		Timestamp:              currentTime,
		SignedCollectionHashes: signedCollectionHashes,
		BlockSeals:             blockSeals,
		Signatures:             sigs,
	}
}

func MockBlockMessage() *bambooProto.Block {
	timestamp, _ := ptypes.TimestampProto(currentTime)
	signedCollectionHashes := make([]*bambooProto.SignedCollectionHash, 0)
	blockSeals := make([]*bambooProto.BlockSeal, 0)
	sigs := make([][]byte, 0)
	for i := 0; i < 5; i++ {
		signedCollectionHashes = append(signedCollectionHashes, MockSignedCollectionHashMessage())
		blockSeals = append(blockSeals, MockBlockSealMessage())
		sigs = append(sigs, MockSignature().Bytes())
	}

	return &bambooProto.Block{
		ChainID:                "BAMBOO",
		Height:                 0,
		PreviousBlockHash:      MockHash().Bytes(),
		Timestamp:              timestamp,
		SignedCollectionHashes: signedCollectionHashes,
		BlockSeals:             blockSeals,
		Signatures:             sigs,
	}
}
