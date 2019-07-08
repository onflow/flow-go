package utils

import (

	"github.com/golang/protobuf/ptypes"

	bambooProto "github.com/dapperlabs/bamboo-node/grpc/shared"
	"github.com/dapperlabs/bamboo-node/internal/types"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

func MessageToRegister(m *bambooProto.Register) *types.Register {
	return &types.Register{
		ID:    m.GetId(),
		Value: m.GetValue(),
	}
}

func MessageToIntermediateRegisters(m *bambooProto.IntermediateRegisters) *types.IntermediateRegisters {
	registers := make([]types.Register, 0)
	for _, r := range m.GetRegisters() {
		registers = append(registers, *MessageToRegister(r))
	}

	return &types.IntermediateRegisters{
		TransactionHash: crypto.BytesToHash(m.GetTransactionHash()),
		Registers:       registers,
		ComputeUsed:     m.GetComputeUsed(),
	}
}

func MessageToTransactionRegister(m *bambooProto.TransactionRegister) *types.TransactionRegister {
	return &types.TransactionRegister{}
}

func MessageToCollection(m *bambooProto.Collection) *types.Collection {
	transactions := make([]types.SignedTransaction, 0)
	for _, tx := range m.GetTransactions() {
		transactions = append(transactions, *MessageToSignedTransaction(tx))
	}

	return &types.Collection{
		Transactions:        transactions,
		FoundationBlockHash: crypto.BytesToHash(m.GetFoundationBlockHash()),
	}
}

func MessageToSignedCollectionHash(m *bambooProto.SignedCollectionHash) *types.SignedCollectionHash {
	sigs := make([]crypto.Signature, 0)
	for _, sig := range m.GetSignatures() {
		sigs = append(sigs, crypto.BytesToSig(sig))
	}

	return &types.SignedCollectionHash{
		CollectionHash: crypto.BytesToHash(m.GetCollectionHash()),
		Signatures:     sigs,
	}
}

func MessageToBlock(m *bambooProto.Block) *types.Block {
	ts, _ := ptypes.Timestamp(m.GetTimestamp())

	collectionHashes := make([]types.SignedCollectionHash, 0)
	for _, hash := range m.GetSignedCollectionHashes() {
		collectionHashes = append(collectionHashes, *MessageToSignedCollectionHash(hash))
	}

	blockSeals := make([]types.BlockSeal, 0)
	for _, seal := range m.GetBlockSeals() {
		blockSeals = append(blockSeals, *MessageToBlockSeal(seal))
	}

	sigs := make([]crypto.Signature, 0)
	for _, sig := range m.GetSignatures() {
		sigs = append(sigs, crypto.BytesToSig(sig))
	}

	return &types.Block{
		ChainID:                m.GetChainID(),
		Height:                 m.GetHeight(),
		PreviousBlockHash:      crypto.BytesToHash(m.GetPreviousBlockHash()),
		Timestamp:              ts,
		SignedCollectionHashes: collectionHashes,
		BlockSeals:             blockSeals,
		Signatures:             sigs,
	}
}

func MessageToBlockSeal(m *bambooProto.BlockSeal) *types.BlockSeal {
	erSigs := make([]crypto.Signature, 0)
	for _, sig := range m.GetExecutionReceiptSignatures() {
		erSigs = append(erSigs, crypto.BytesToSig(sig))
	}

	raSigs := make([]crypto.Signature, 0)
	for _, sig := range m.GetResultApprovalSignatures() {
		raSigs = append(raSigs, crypto.BytesToSig(sig))
	}

	return &types.BlockSeal{
		BlockHash:                  crypto.BytesToHash(m.GetBlockHash()),
		ExecutionReceiptHash:       crypto.BytesToHash(m.GetExecutionReceiptHash()),
		ExecutionReceiptSignatures: erSigs,
		ResultApprovalSignatures:   raSigs,
	}
}

func MessageToTransaction(m *bambooProto.Transaction) *types.Transaction {
	registers := make([]types.TransactionRegister, 0)
	for _, r := range m.GetRegisters() {
		registers = append(registers, *MessageToTransactionRegister(r))
	}

	return &types.Transaction{
		Script:    m.GetScript(),
		Nonce:     m.GetNonce(),
		Registers: registers,
		Chunks:    m.GetChunks(),
	}
}

func MessageToSignedTransaction(m *bambooProto.SignedTransaction) *types.SignedTransaction {
	sigs := make([]crypto.Signature, 0)
	for _, sig := range m.GetScriptSignatures() {
		sigs = append(sigs, crypto.BytesToSig(sig))
	}

	return &types.SignedTransaction{
		Transaction:      *MessageToTransaction(m.GetTransaction()),
		ScriptSignatures: sigs,
		PayerSignature:   crypto.BytesToSig(m.GetPayerSignature()),
	}
}

func MessageToExecutionReceipt(m *bambooProto.ExecutionReceipt) *types.ExecutionReceipt {
	registers := make([]types.Register, 0)
	for _, r := range m.GetInitialRegisters() {
		registers = append(registers, *MessageToRegister(r))
	}

	irList := make([]types.IntermediateRegisters, 0)
	for _, ir := range m.GetIntermediateRegistersList() {
		irList = append(irList, *MessageToIntermediateRegisters(ir))
	}

	sigs := make([]crypto.Signature, 0)
	for _, sig := range m.GetSignatures() {
		sigs = append(sigs, crypto.BytesToSig(sig))
	}

	return &types.ExecutionReceipt{
		PreviousReceiptHash:       crypto.BytesToHash(m.GetPreviousReceiptHash()),
		BlockHash:                 crypto.BytesToHash(m.GetBlockHash()),
		InitialRegisters:          registers,
		IntermediateRegistersList: irList,
		Signatures:                sigs,
	}
}

func MessageToInvalidExecutionReceiptChallenge(m *bambooProto.InvalidExecutionReceiptChallenge) *types.InvalidExecutionReceiptChallenge {
	partTransactions := make([]types.IntermediateRegisters, 0)
	for _, ir := range m.GetPartTransactions() {
		partTransactions = append(partTransactions, *MessageToIntermediateRegisters(ir))
	}

	return &types.InvalidExecutionReceiptChallenge{
		ExecutionReceiptHash:      crypto.BytesToHash(m.GetExecutionReceiptHash()),
		ExecutionReceiptSignature: crypto.BytesToSig(m.GetExecutionReceiptSignature()),
		PartIndex:                 m.GetPartIndex(),
		PartTransactions:          partTransactions,
		Signature:                 crypto.BytesToSig(m.GetSignature()),
	}
}

func MessageToResultApproval(m *bambooProto.ResultApproval) *types.ResultApproval {
	return &types.ResultApproval{
		BlockHeight:             m.GetBlockHeight(),
		ExecutionReceiptHash:    crypto.BytesToHash(m.GetExecutionReceiptHash()),
		ResultApprovalSignature: crypto.BytesToSig(m.GetResultApprovalSignature()),
		Proof:                   m.GetProof(),
		Signature:               crypto.BytesToSig(m.GetSignature()),
	}
}
