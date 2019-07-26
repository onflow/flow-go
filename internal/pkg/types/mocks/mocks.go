package types

import (
	"time"

	"github.com/golang/protobuf/ptypes"

	"github.com/dapperlabs/bamboo-node/internal/pkg/types"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/pkg/grpc/shared"
)

var mockedCurrentTime = time.Date(2009, 11, 17, 20, 34, 58, 651387237, time.UTC)

func MockHash() crypto.Hash {
	return crypto.Hash{}
}

func MockSignature() crypto.Signature {
	return crypto.Signature{}
}

func MockKeyWeight() types.KeyWeight {
	return types.KeyWeight{
		Key:    []byte("MOCK"),
		Weight: 0,
	}
}

func MockKeyWeightMessage() *shared.TransactionRegister_KeyWeight {
	return &shared.TransactionRegister_KeyWeight{
		Key:    []byte("MOCK"),
		Weight: 0,
	}
}

func MockRegister() *types.Register {
	return &types.Register{
		ID:    []byte("TEST"),
		Value: []byte("MOCK"),
	}
}

func MockRegisterMessage() *shared.Register {
	return &shared.Register{
		Id:    []byte("TEST"),
		Value: []byte("MOCK"),
	}
}

func MockIntermediateRegisters() *types.IntermediateRegisters {
	registers := make([]types.Register, 0)
	for i := 0; i < 5; i++ {
		registers = append(registers, *MockRegister())
	}

	return &types.IntermediateRegisters{
		TransactionHash: MockHash(),
		Registers:       registers,
		ComputeUsed:     0,
	}
}

func MockIntermediateRegistersMessage() *shared.IntermediateRegisters {
	registers := make([]*shared.Register, 0)
	for i := 0; i < 5; i++ {
		registers = append(registers, MockRegisterMessage())
	}

	return &shared.IntermediateRegisters{
		TransactionHash: MockHash().Bytes(),
		Registers:       registers,
		ComputeUsed:     0,
	}
}

func MockTransactionRegister() *types.TransactionRegister {
	keys := make([]types.KeyWeight, 0)
	for i := 0; i < 5; i++ {
		keys = append(keys, MockKeyWeight())
	}

	return &types.TransactionRegister{
		Type:       types.SIMPLE,
		AccessMode: types.CREATE,
		ID:         []byte("TEST"),
		Keys:       keys,
	}
}

func MockTransactionRegisterMessage() *shared.TransactionRegister {
	keys := make([]*shared.TransactionRegister_KeyWeight, 0)
	for i := 0; i < 5; i++ {
		keys = append(keys, MockKeyWeightMessage())
	}

	return &shared.TransactionRegister{
		Type:       shared.TransactionRegister_SIMPLE,
		AccessMode: shared.TransactionRegister_CREATE,
		Id:         []byte("TEST"),
		Keys:       keys,
	}
}

func MockTransaction() *types.Transaction {
	registers := make([]types.TransactionRegister, 0)
	for i := 0; i < 5; i++ {
		registers = append(registers, *MockTransactionRegister())
	}

	return &types.Transaction{
		Script:    []byte("TEST"),
		Nonce:     0,
		Registers: registers,
		Chunks:    make([][]byte, 0),
	}
}

func MockTransactionMessage() *shared.Transaction {
	registers := make([]*shared.TransactionRegister, 0)
	for i := 0; i < 5; i++ {
		registers = append(registers, MockTransactionRegisterMessage())
	}

	return &shared.Transaction{
		Script:    []byte("TEST"),
		Nonce:     0,
		Registers: registers,
		Chunks:    make([][]byte, 0),
	}
}

func MockSignedTransaction() *types.SignedTransaction {
	sigs := make([]crypto.Signature, 0)
	for i := 0; i < 5; i++ {
		sigs = append(sigs, MockSignature())
	}

	return &types.SignedTransaction{
		Transaction:      *MockTransaction(),
		ScriptSignatures: sigs,
		PayerSignature:   MockSignature(),
	}
}

func MockSignedTransactionMessage() *shared.SignedTransaction {
	sigs := make([][]byte, 0)
	for i := 0; i < 5; i++ {
		sigs = append(sigs, MockSignature().Bytes())
	}

	return &shared.SignedTransaction{
		Transaction:      MockTransactionMessage(),
		ScriptSignatures: sigs,
		PayerSignature:   MockSignature().Bytes(),
	}
}

func MockCollection() *types.Collection {
	transactions := make([]SignedTransaction, 0)
	for i := 0; i < 5; i++ {
		transactions = append(transactions, *MockSignedTransaction())
	}

	return &types.Collection{
		Transactions:        transactions,
		FoundationBlockHash: MockHash(),
	}
}

func MockCollectionMessage() *shared.Collection {
	transactions := make([]*shared.SignedTransaction, 0)
	for i := 0; i < 5; i++ {
		transactions = append(transactions, MockSignedTransactionMessage())
	}

	return &shared.Collection{
		Transactions:        transactions,
		FoundationBlockHash: MockHash().Bytes(),
	}
}

func MockSignedCollectionHash() *types.SignedCollectionHash {
	sigs := make([]crypto.Signature, 0)
	for i := 0; i < 5; i++ {
		sigs = append(sigs, MockSignature())
	}

	return &types.SignedCollectionHash{
		CollectionHash: MockHash(),
		Signatures:     sigs,
	}
}

func MockSignedCollectionHashMessage() *shared.SignedCollectionHash {
	sigs := make([][]byte, 0)
	for i := 0; i < 5; i++ {
		sigs = append(sigs, MockSignature().Bytes())
	}

	return &shared.SignedCollectionHash{
		CollectionHash: MockHash().Bytes(),
		Signatures:     sigs,
	}
}

func MockExecutionReceipt() *types.ExecutionReceipt {
	registers := make([]types.Register, 0)
	irList := make([]types.IntermediateRegisters, 0)
	sigs := make([]crypto.Signature, 0)
	for i := 0; i < 5; i++ {
		registers = append(registers, *MockRegister())
		irList = append(irList, *MockIntermediateRegisters())
		sigs = append(sigs, MockSignature())
	}

	return &types.ExecutionReceipt{
		PreviousReceiptHash:       MockHash(),
		BlockHash:                 MockHash(),
		InitialRegisters:          registers,
		IntermediateRegistersList: irList,
		Signatures:                sigs,
	}
}

func MockExecutionReceiptMessage() *shared.ExecutionReceipt {
	registers := make([]*shared.Register, 0)
	irList := make([]*shared.IntermediateRegisters, 0)
	sigs := make([][]byte, 0)
	for i := 0; i < 5; i++ {
		registers = append(registers, MockRegisterMessage())
		irList = append(irList, MockIntermediateRegistersMessage())
		sigs = append(sigs, MockSignature().Bytes())
	}

	return &shared.ExecutionReceipt{
		PreviousReceiptHash:       MockHash().Bytes(),
		BlockHash:                 MockHash().Bytes(),
		InitialRegisters:          registers,
		IntermediateRegistersList: irList,
		Signatures:                sigs,
	}
}

func MockInvalidExecutionReceiptChallenge() *types.InvalidExecutionReceiptChallenge {
	partTransactions := make([]types.IntermediateRegisters, 0)
	for i := 0; i < 5; i++ {
		partTransactions = append(partTransactions, *MockIntermediateRegisters())
	}

	return &types.InvalidExecutionReceiptChallenge{
		ExecutionReceiptHash:      MockHash(),
		ExecutionReceiptSignature: MockSignature(),
		PartIndex:                 0,
		PartTransactions:          partTransactions,
		Signature:                 MockSignature(),
	}
}

func MockInvalidExecutionReceiptChallengeMessage() *shared.InvalidExecutionReceiptChallenge {
	partTransactions := make([]*shared.IntermediateRegisters, 0)
	for i := 0; i < 5; i++ {
		partTransactions = append(partTransactions, MockIntermediateRegistersMessage())
	}

	return &shared.InvalidExecutionReceiptChallenge{
		ExecutionReceiptHash:      MockHash().Bytes(),
		ExecutionReceiptSignature: MockSignature().Bytes(),
		PartIndex:                 0,
		PartTransactions:          partTransactions,
		Signature:                 MockSignature().Bytes(),
	}
}

func MockResultApproval() *types.ResultApproval {
	return &types.ResultApproval{
		BlockHeight:             0,
		ExecutionReceiptHash:    MockHash(),
		ResultApprovalSignature: MockSignature(),
		Proof:                   0,
		Signature:               MockSignature(),
	}
}

func MockResultApprovalMessage() *shared.ResultApproval {
	return &shared.ResultApproval{
		BlockHeight:             0,
		ExecutionReceiptHash:    MockHash().Bytes(),
		ResultApprovalSignature: MockSignature().Bytes(),
		Proof:                   0,
		Signature:               MockSignature().Bytes(),
	}
}

func MockBlockSeal() *types.BlockSeal {
	erSigs := make([]crypto.Signature, 0)
	raSigs := make([]crypto.Signature, 0)
	for i := 0; i < 5; i++ {
		erSigs = append(erSigs, MockSignature())
		raSigs = append(raSigs, MockSignature())
	}

	return &types.BlockSeal{
		BlockHash:                  MockHash(),
		ExecutionReceiptHash:       MockHash(),
		ExecutionReceiptSignatures: erSigs,
		ResultApprovalSignatures:   raSigs,
	}
}

func MockBlockSealMessage() *shared.BlockSeal {
	erSigs := make([][]byte, 0)
	raSigs := make([][]byte, 0)
	for i := 0; i < 5; i++ {
		erSigs = append(erSigs, MockSignature().Bytes())
		raSigs = append(raSigs, MockSignature().Bytes())
	}

	return &shared.BlockSeal{
		BlockHash:                  MockHash().Bytes(),
		ExecutionReceiptHash:       MockHash().Bytes(),
		ExecutionReceiptSignatures: erSigs,
		ResultApprovalSignatures:   raSigs,
	}
}

func MockBlock() *types.Block {
	signedCollectionHashes := make([]types.SignedCollectionHash, 0)
	blockSeals := make([]types.BlockSeal, 0)
	sigs := make([]crypto.Signature, 0)
	for i := 0; i < 5; i++ {
		signedCollectionHashes = append(signedCollectionHashes, *MockSignedCollectionHash())
		blockSeals = append(blockSeals, *MockBlockSeal())
		sigs = append(sigs, MockSignature())
	}

	return &types.Block{
		ChainID:                "BAMBOO",
		Height:                 0,
		PreviousBlockHash:      MockHash(),
		Timestamp:              mockedCurrentTime,
		SignedCollectionHashes: signedCollectionHashes,
		BlockSeals:             blockSeals,
		Signatures:             sigs,
	}
}

func MockBlockMessage() *shared.Block {
	timestamp, _ := ptypes.TimestampProto(mockedCurrentTime)
	signedCollectionHashes := make([]*shared.SignedCollectionHash, 0)
	blockSeals := make([]*shared.BlockSeal, 0)
	sigs := make([][]byte, 0)
	for i := 0; i < 5; i++ {
		signedCollectionHashes = append(signedCollectionHashes, MockSignedCollectionHashMessage())
		blockSeals = append(blockSeals, MockBlockSealMessage())
		sigs = append(sigs, MockSignature().Bytes())
	}

	return &shared.Block{
		ChainID:                "BAMBOO",
		Height:                 0,
		PreviousBlockHash:      MockHash().Bytes(),
		Timestamp:              timestamp,
		SignedCollectionHashes: signedCollectionHashes,
		BlockSeals:             blockSeals,
		Signatures:             sigs,
	}
}

func MockStateTransition() *types.StateTransition {
	sigs := make([]crypto.Signature, 0)
	for i := 0; i < 5; i++ {
		sigs = append(sigs, MockSignature())
	}

	return &types.StateTransition{
		PreviousStateTransitionHash:      MockHash(),
		PreviousCommitApprovalSignatures: sigs,
		Height:                           0,
		Value:                            []byte("MOCK"),
	}
}

func MockStateTransitionMessage() *shared.StateTransition {
	sigs := make([][]byte, 0)
	for i := 0; i < 5; i++ {
		sigs = append(sigs, MockSignature().Bytes())
	}

	return &shared.StateTransition{
		PreviousStateTransitionHash:      MockHash().Bytes(),
		PreviousCommitApprovalSignatures: sigs,
		Height:                           0,
		Value:                            []byte("MOCK"),
	}
}

func MockSignedStateTransition() *types.SignedStateTransition {
	return &types.SignedStateTransition{
		StateTransition: *MockStateTransition(),
		Signature:       MockSignature(),
	}
}

func MockSignedStateTransitionMessage() *shared.SignedStateTransition {
	return &shared.SignedStateTransition{
		StateTransition: MockStateTransitionMessage(),
		Signature:       MockSignature().Bytes(),
	}
}

func MockFinalizedStateTransition() *types.FinalizedStateTransition {
	sigs := make([]crypto.Signature, 0)
	for i := 0; i < 5; i++ {
		sigs = append(sigs, MockSignature())
	}

	return &types.FinalizedStateTransition{
		SignedStateTransition: *MockSignedStateTransition(),
		Signatures:            sigs,
	}
}

func MockFinalizedStateTransitionMessage() *shared.FinalizedStateTransition {
	sigs := make([][]byte, 0)
	for i := 0; i < 5; i++ {
		sigs = append(sigs, MockSignature().Bytes())
	}

	return &shared.FinalizedStateTransition{
		SignedStateTransition: MockSignedStateTransitionMessage(),
		Signatures:            sigs,
	}
}

func MockStateTransitionVote() *types.StateTransitionVote {
	return &types.StateTransitionVote{
		StateTransitionHash: MockHash(),
		Vote:                APPROVE,
		Height:              0,
	}
}

func MockStateTransitionVoteMessage() *shared.StateTransitionVote {
	return &shared.StateTransitionVote{
		StateTransitionHash: MockHash().Bytes(),
		Vote:                shared.Vote_APPROVE,
		Height:              0,
	}
}
