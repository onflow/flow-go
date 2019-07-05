package utils

import (
	bambooProto "github.com/dapperlabs/bamboo-node/grpc/shared"
	"github.com/dapperlabs/bamboo-node/internal/types"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

func (m *bambooProto.Register) FromMessage() *types.Register {
	return &types.Register{
		ID:    m.GetId(),
		Value: m.GetValue(),
	}
}

func (t *types.Register) ToMessage() *bambooProto.Register {
	return &bambooProto.Register{
		Id:    t.ID,
		Value: t.Value,
	}
}

func (m *bambooProto.IntermediateRegisters) FromMessage() *types.IntermediateRegisters {
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

func (t *types.IntermediateRegisters) ToMessage() *bambooProto.IntermediateRegisters {
	registers := make([]*bambooProto.Register, 0)
	for _, r := range t.Registers {
		registers = append(registers, r.ToMessage())
	}

	return &bambooProto.IntermediateRegisters{
		TransactionHash: t.TransactionHash.Bytes(),
		Registers:       registers,
		ComputeUsed:     t.ComputeUsed,
	}
}

func (m *bambooProto.TransactionRegister) FromMessage() *types.TransactionRegister {
	return &types.TransactionRegister{}
}

func (t *types.TransactionRegister) ToMessage() *bambooProto.TransactionRegister {
	return &bambooProto.TransactionRegister{}
}

func (m *bambooProto.Collection) FromMessage() *types.Collection {
	return &types.Collection{}
}

func (t *types.Collection) ToMessage() *bambooProto.Collection {
	return &bambooProto.Collection{}
}

func (m *bambooProto.SignedCollectionHash) FromMessage() *types.SignedCollectionHash {
	return &types.SignedCollectionHash{}
}

func (t *types.SignedCollectionHash) ToMessage() *bambooProto.SignedCollectionHash {
	return &bambooProto.SignedCollectionHash{}
}

func (m *bambooProto.Block) FromMessage() *types.Block {
	return &types.Block{}
}

func (t *types.Block) ToMessage() *bambooProto.Block {
	return &bambooProto.Block{}
}

func (m *bambooProto.BlockSeal) FromMessage() *types.BlockSeal {
	return &types.BlockSeal{}
}

func (t *types.BlockSeal) ToMessage() *bambooProto.BlockSeal {
	return &bambooProto.BlockSeal{}
}

func (m *bambooProto.Transaction) FromMessage() *types.Transaction {
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

func (t *types.Transaction) ToMessage() *bambooProto.Transaction {
	registers := make([]*bambooProto.TransactionRegister, 0)
	for _, r := range t.Registers {
		registers = append(registers, r.ToMessage())
	}

	return &bambooProto.Transaction{
		Script:    t.Script,
		Nonce:     t.Nonce,
		Registers: registers,
		Chunks:    t.Chunks,
	}
}

func (m *bambooProto.SignedTransaction) FromMessage() *types.SignedTransaction {
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

func (t *types.SignedTransaction) ToMessage() *bambooProto.SignedTransaction {
	sigs := make([][]byte, 0)
	for _, sig := range t.ScriptSignatures {
		sigs = append(sigs, sig.Bytes())
	}

	return &bambooProto.SignedTransaction{
		Transaction:      t.Transaction.ToMessage(),
		ScriptSignatures: sigs,
		PayerSignature:   t.PayerSignature.Bytes(),
	}
}

func (m *bambooProto.ExecutionReceipt) FromMessage() *types.ExecutionReceipt {
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

func (t *types.ExecutionReceipt) ToMessage() *bambooProto.ExecutionReceipt {
	registers := make([]*bambooProto.Register, 0)
	for _, r := range t.InitialRegisters {
		registers = append(registers, r.ToMessage())
	}

	irList := make([]*bambooProto.IntermediateRegisters, 0)
	for _, ir := range t.IntermediateRegistersList {
		irList = append(irList, ir.ToMessage())
	}

	sigs := make([][]byte, 0)
	for _, sig := range t.Signatures {
		sigs = append(sigs, sig.Bytes())
	}

	return &bambooProto.ExecutionReceipt{
		PreviousReceiptHash:       t.PreviousReceiptHash.Bytes(),
		BlockHash:                 t.BlockHash.Bytes(),
		InitialRegisters:          registers,
		IntermediateRegistersList: irList,
		Signatures:                sigs,
	}
}

func (m *bambooProto.InvalidExecutionReceiptChallenge) FromMessage() *types.InvalidExecutionReceiptChallenge {
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

func (t *types.InvalidExecutionReceiptChallenge) ToMessage() *bambooProto.InvalidExecutionReceiptChallenge {
	partTransactions := make([]*bambooProto.IntermediateRegisters, 0)
	for _, ir := range t.PartTransactions {
		partTransactions = append(partTransactions, ir.ToMessage())
	}

	return &bambooProto.InvalidExecutionReceiptChallenge{
		ExecutionReceiptHash:      t.ExecutionReceiptHash.Bytes(),
		ExecutionReceiptSignature: t.ExecutionReceiptSignature.Bytes(),
		PartIndex:                 t.PartIndex,
		PartTransactions:          partTransactions,
		Signature:                 t.Signature.Bytes(),
	}
}

func (m *bambooProto.ResultApproval) FromMessage() *types.ResultApproval {
	return &types.ResultApproval{
		BlockHeight:             m.GetBlockHeight(),
		ExecutionReceiptHash:    crypto.BytesToHash(m.GetExecutionReceiptHash()),
		ResultApprovalSignature: crypto.BytesToSig(m.GetResultApprovalSignature()),
		Proof:                   m.GetProof(),
		Signature:               crypto.BytesToSig(m.GetSignature()),
	}
}

func (t *types.ResultApproval) ToMessage() *bambooProto.ResultApproval {
	return &bambooProto.ResultApproval{
		BlockHeight:             t.BlockHeight,
		ExecutionReceiptHash:    t.ExecutionReceiptHash.Bytes(),
		ResultApprovalSignature: t.ResultApprovalSignature.Bytes(),
		Proof:                   t.Proof,
		Signature:               t.Signature.Bytes(),
	}
}
