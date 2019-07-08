package shared

import (
	"github.com/golang/protobuf/ptypes"

	"github.com/dapperlabs/bamboo-node/internal/types"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

func (m *Register) FromMessage() *types.Register {
	return &types.Register{
		ID:    m.GetId(),
		Value: m.GetValue(),
	}
}

func (m *IntermediateRegisters) FromMessage() *types.IntermediateRegisters {
	registers := make([]types.Register, 0)
	for _, r := range m.GetRegisters() {
		registers = append(registers, *r.FromMessage())
	}

	return &types.IntermediateRegisters{
		TransactionHash: crypto.BytesToHash(m.GetTransactionHash()),
		Registers:       registers,
		ComputeUsed:     m.GetComputeUsed(),
	}
}

func (m *TransactionRegister) FromMessage() *types.TransactionRegister {
	return &types.TransactionRegister{}
}

func (m *Collection) FromMessage() *types.Collection {
	transactions := make([]types.SignedTransaction, 0)
	for _, tx := range m.GetTransactions() {
		transactions = append(transactions, *tx.FromMessage())
	}

	return &types.Collection{
		Transactions:        transactions,
		FoundationBlockHash: crypto.BytesToHash(m.GetFoundationBlockHash()),
	}
}

func (m *SignedCollectionHash) FromMessage() *types.SignedCollectionHash {
	sigs := make([]crypto.Signature, 0)
	for _, sig := range m.GetSignatures() {
		sigs = append(sigs, crypto.BytesToSig(sig))
	}

	return &types.SignedCollectionHash{
		CollectionHash: crypto.BytesToHash(m.GetCollectionHash()),
		Signatures:     sigs,
	}
}

func (m *Block) FromMessage() *types.Block {
	ts, _ := ptypes.Timestamp(m.GetTimestamp())

	collectionHashes := make([]types.SignedCollectionHash, 0)
	for _, hash := range m.GetSignedCollectionHashes() {
		collectionHashes = append(collectionHashes, *hash.FromMessage())
	}

	blockSeals := make([]types.BlockSeal, 0)
	for _, seal := range m.GetBlockSeals() {
		blockSeals = append(blockSeals, *seal.FromMessage())
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

func (m *BlockSeal) FromMessage() *types.BlockSeal {
	erSigs := make([]crypto.Signature, 0)
	for _, sig := range m.GetExecutionReceiptSignatures() {
		erSigs = append(erSigs, crypto.BytesToSig(sig))
	}

	raSigs := make([]crypto.Signature, 0)
	for _, sig := range m.GetResultApprovalSignatures() {
		sigs = append(sigs, crypto.BytesToSig(sig))
	}

	return &types.BlockSeal{
		BlockHash:                  crypto.BytesToHash(m.GetBlockHash()),
		ExecutionReceiptHash:       crypto.BytesToHash(m.GetExecutionReceiptHash()),
		ExecutionReceiptSignatures: erSigs,
		ResultApprovalSignatures:   raSigs,
	}
}

func (m *Transaction) FromMessage() *types.Transaction {
	registers := make([]types.TransactionRegister, 0)
	for _, r := range m.GetRegisters() {
		registers = append(registers, *r.FromMessage())
	}

	return &types.Transaction{
		Script:    m.GetScript(),
		Nonce:     m.GetNonce(),
		Registers: registers,
		Chunks:    m.GetChunks(),
	}
}

func (m *SignedTransaction) FromMessage() *types.SignedTransaction {
	sigs := make([]crypto.Signature, 0)
	for _, sig := range m.GetScriptSignatures() {
		sigs = append(sigs, crypto.BytesToSig(sig))
	}

	return &types.SignedTransaction{
		Transaction:      *m.GetTransaction().FromMessage(),
		ScriptSignatures: sigs,
		PayerSignature:   crypto.BytesToSig(m.GetPayerSignature()),
	}
}

func (m *ExecutionReceipt) FromMessage() *types.ExecutionReceipt {
	registers := make([]types.Register, 0)
	for _, r := range m.GetInitialRegisters() {
		registers = append(registers, *r.FromMessage())
	}

	irList := make([]types.IntermediateRegisters, 0)
	for _, ir := range m.GetIntermediateRegistersList() {
		irList = append(irList, *ir.FromMessage())
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

func (m *InvalidExecutionReceiptChallenge) FromMessage() *types.InvalidExecutionReceiptChallenge {
	partTransactions := make([]types.IntermediateRegisters, 0)
	for _, ir := range m.GetPartTransactions() {
		partTransactions = append(partTransactions, *ir.FromMessage())
	}

	return &types.InvalidExecutionReceiptChallenge{
		ExecutionReceiptHash:      crypto.BytesToHash(m.GetExecutionReceiptHash()),
		ExecutionReceiptSignature: crypto.BytesToSig(m.GetExecutionReceiptSignature()),
		PartIndex:                 m.GetPartIndex(),
		PartTransactions:          partTransactions,
		Signature:                 crypto.BytesToSig(m.GetSignature()),
	}
}

func (m *ResultApproval) FromMessage() *types.ResultApproval {
	return &types.ResultApproval{
		BlockHeight:             m.GetBlockHeight(),
		ExecutionReceiptHash:    crypto.BytesToHash(m.GetExecutionReceiptHash()),
		ResultApprovalSignature: crypto.BytesToSig(m.GetResultApprovalSignature()),
		Proof:                   m.GetProof(),
		Signature:               crypto.BytesToSig(m.GetSignature()),
	}
}
