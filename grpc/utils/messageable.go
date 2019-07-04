package utils

import (
	bambooProto "github.com/dapperlabs/bamboo-node/grpc/shared"
	"github.com/dapperlabs/bamboo-node/internal/types"
)

func (m *bambooProto.Register) FromMessage() *types.Register {
	return &types.Register{
		ID:    m.GetId(),
		Value: m.GetValue(),
	}
}

func (b *types.Register) ToMessage() *bambooProto.Register {
	return &bambooProto.Register{
		Id:    b.ID,
		Value: b.Value,
	}
}

func (m *bambooProto.IntermediateRegisters) FromMessage() *types.IntermediateRegisters {
	return &types.IntermediateRegisters{}
}

func (b *types.IntermediateRegisters) ToMessage() *bambooProto.IntermediateRegisters {
	return &bambooProto.IntermediateRegisters{}
}

func (m *bambooProto.TransactionRegister) FromMessage() *types.TransactionRegister {
	return &types.TransactionRegister{}
}

func (b *types.TransactionRegister) ToMessage() *bambooProto.TransactionRegister {
	return &bambooProto.TransactionRegister{}
}

func (m *bambooProto.Collection) FromMessage() *types.Collection {
	return &types.Collection{}
}

func (b *types.Collection) ToMessage() *bambooProto.Collection {
	return &bambooProto.Collection{}
}

func (m *bambooProto.SignedCollectionHash) FromMessage() *types.SignedCollectionHash {
	return &types.SignedCollectionHash{}
}

func (b *types.SignedCollectionHash) ToMessage() *bambooProto.SignedCollectionHash {
	return &bambooProto.SignedCollectionHash{}
}

func (m *bambooProto.Block) FromMessage() *types.Block {
	return &types.Block{}
}

func (b *types.Block) ToMessage() *bambooProto.Block {
	return &bambooProto.Block{}
}

func (m *bambooProto.BlockSeal) FromMessage() *types.BlockSeal {
	return &types.BlockSeal{}
}

func (b *types.BlockSeal) ToMessage() *bambooProto.BlockSeal {
	return &bambooProto.BlockSeal{}
}

func (m *bambooProto.Transaction) FromMessage() *types.Transaction {
	return &types.Transaction{}
}

func (b *types.Transaction) ToMessage() *bambooProto.Transaction {
	return &bambooProto.Transaction{}
}

func (m *bambooProto.SignedTransaction) FromMessage() *types.SignedTransaction {
	return &types.SignedTransaction{}
}

func (b *types.SignedTransaction) ToMessage() *bambooProto.SignedTransaction {
	return &bambooProto.SignedTransaction{}
}

func (m *bambooProto.ExecutionReceipt) FromMessage() *types.ExecutionReceipt {
	return &types.ExecutionReceipt{}
}

func (b *types.ExecutionReceipt) ToMessage() *bambooProto.ExecutionReceipt {
	return &bambooProto.ExecutionReceipt{}
}

func (m *bambooProto.InvalidExecutionReceiptChallenge) FromMessage() *types.InvalidExecutionReceiptChallenge {
	return &types.InvalidExecutionReceiptChallenge{}
}

func (b *types.InvalidExecutionReceiptChallenge) ToMessage() *bambooProto.InvalidExecutionReceiptChallenge {
	return &bambooProto.InvalidExecutionReceiptChallenge{}
}

func (m *bambooProto.ResultApproval) FromMessage() *types.ResultApproval {
	return &types.ResultApproval{}
}

func (b *types.ResultApproval) ToMessage() *bambooProto.ResultApproval {
	return &bambooProto.ResultApproval{}
}
