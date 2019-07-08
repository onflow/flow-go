package types

import (
	"github.com/golang/protobuf/ptypes"

	bambooProto "github.com/dapperlabs/bamboo-node/grpc/shared"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

func (t *Register) ToMessage() *bambooProto.Register {
	return &bambooProto.Register{
		Id:    t.ID,
		Value: t.Value,
	}
}

func (t *IntermediateRegisters) ToMessage() *bambooProto.IntermediateRegisters {
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

func (t *TransactionRegister) ToMessage() *bambooProto.TransactionRegister {
	return &bambooProto.TransactionRegister{}
}

func (t *Collection) ToMessage() *bambooProto.Collection {
	transactions := make([]*bambooProto.SignedTransaction, 0)
	for _, tx := range t.Transactions {
		transactions = append(transactions, tx.ToMessage())
	}

	return &bambooProto.Collection{
		Transactions:        transactions,
		FoundationBlockHash: t.FoundationBlockHash.Bytes(),
	}
}

func (t *SignedCollectionHash) ToMessage() *bambooProto.SignedCollectionHash {
	sigs := make([][]byte, 0)
	for _, sig := range t.Signatures {
		sigs = append(sigs, sig.Bytes())
	}

	return &bambooProto.SignedCollectionHash{
		CollectionHash: t.CollectionHash.Bytes(),
		Signatures:     sigs,
	}
}

func (t *Block) ToMessage() *bambooProto.Block {
	ts, _ := ptypes.TimestampProto(t.Timestamp)

	collectionHashes := make([]*bambooProto.SignedCollectionHash, 0)
	for _, hash := range t.SignedCollectionHashes {
		collectionHashes = append(collectionHashes, hash.ToMessage())
	}

	blockSeals := make([]*bambooProto.BlockSeal, 0)
	for _, seal := range t.BlockSeals {
		blockSeals = append(blockSeals, seal.ToMessage())
	}

	sigs := make([][]byte, 0)
	for _, sig := range t.Signatures {
		sigs = append(sigs, sig.Bytes())
	}

	return &bambooProto.Block{
		ChainID:                t.ChainID,
		Height:                 t.Height,
		PreviousBlockHash:      t.PreviousBlockHash.Bytes(),
		Timestamp:              ts,
		SignedCollectionHashes: collectionHashes,
		BlockSeals:             blockSeals,
		Signatures:             sigs,
	}
}

func (t *BlockSeal) ToMessage() *bambooProto.BlockSeal {
	erSigs := make([][]byte, 0)
	for _, sig := range t.ExecutionReceiptSignatures {
		erSigs = append(erSigs, sig.Bytes())
	}

	raSigs := make([][]byte, 0)
	for _, sig := range t.ResultApprovalSignatures {
		sigs = append(sigs, sig.Bytes())
	}

	return &bambooProto.BlockSeal{
		BlockHash:                  crypto.BlockHash.Bytes(),
		ExecutionReceiptHash:       crypto.ExecutionReceiptHash.Bytes(),
		ExecutionReceiptSignatures: erSigs,
		ResultApprovalSignatures:   raSigs,
	}
}

func (t *Transaction) ToMessage() *bambooProto.Transaction {
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

func (t *SignedTransaction) ToMessage() *bambooProto.SignedTransaction {
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

func (t *ExecutionReceipt) ToMessage() *bambooProto.ExecutionReceipt {
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

func (t *InvalidExecutionReceiptChallenge) ToMessage() *bambooProto.InvalidExecutionReceiptChallenge {
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

func (t *ResultApproval) ToMessage() *bambooProto.ResultApproval {
	return &bambooProto.ResultApproval{
		BlockHeight:             t.BlockHeight,
		ExecutionReceiptHash:    t.ExecutionReceiptHash.Bytes(),
		ResultApprovalSignature: t.ResultApprovalSignature.Bytes(),
		Proof:                   t.Proof,
		Signature:               t.Signature.Bytes(),
	}
}
