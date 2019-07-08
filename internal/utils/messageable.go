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

func RegisterToMessage(t *types.Register) *bambooProto.Register {
	return &bambooProto.Register{
		Id:    t.ID,
		Value: t.Value,
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

func IntermediateRegistersToMessage(t *types.IntermediateRegisters) *bambooProto.IntermediateRegisters {
	registers := make([]*bambooProto.Register, 0)
	for _, r := range t.Registers {
		registers = append(registers, RegisterToMessage(&r))
	}

	return &bambooProto.IntermediateRegisters{
		TransactionHash: t.TransactionHash.Bytes(),
		Registers:       registers,
		ComputeUsed:     t.ComputeUsed,
	}
}

func MessageToKeyWeight(m *bambooProto.TransactionRegister_KeyWeight) *types.KeyWeight {
	return &types.KeyWeight{
		Key:    m.GetKey(),
		Weight: m.GetWeight(),
	}
}

func KeyWeightToMessage(t *types.KeyWeight) *bambooProto.TransactionRegister_KeyWeight {
	return &bambooProto.TransactionRegister_KeyWeight{
		Key:    t.Key,
		Weight: t.Weight,
	}
}

func MessageToTransactionRegister(m *bambooProto.TransactionRegister) *types.TransactionRegister {
	keys := make([]types.KeyWeight, 0)
	for _, key := range m.GetKeys() {
		keys = append(keys, *MessageToKeyWeight(key))
	}

	return &types.TransactionRegister{
		Type:       types.Type(m.GetType()),
		AccessMode: types.AccessMode(m.GetAccessMode()),
		ID:         m.GetId(),
		Keys:       keys,
	}
}

func TransactionRegisterToMessage(t *types.TransactionRegister) *bambooProto.TransactionRegister {
	keys := make([]*bambooProto.TransactionRegister_KeyWeight, 0)
	for _, key := range t.Keys {
		keys = append(keys, KeyWeightToMessage(&key))
	}

	return &bambooProto.TransactionRegister{
		Type:       bambooProto.TransactionRegister_Type(t.Type),
		AccessMode: bambooProto.TransactionRegister_AccessMode(t.AccessMode),
		Id:         t.ID,
		Keys:       keys,
	}
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

func CollectionToMessage(t *types.Collection) *bambooProto.Collection {
	transactions := make([]*bambooProto.SignedTransaction, 0)
	for _, tx := range t.Transactions {
		transactions = append(transactions, SignedTransactionToMessage(&tx))
	}

	return &bambooProto.Collection{
		Transactions:        transactions,
		FoundationBlockHash: t.FoundationBlockHash.Bytes(),
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

func SignedCollectionHashToMessage(t *types.SignedCollectionHash) *bambooProto.SignedCollectionHash {
	sigs := make([][]byte, 0)
	for _, sig := range t.Signatures {
		sigs = append(sigs, sig.Bytes())
	}

	return &bambooProto.SignedCollectionHash{
		CollectionHash: t.CollectionHash.Bytes(),
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

func BlockToMessage(t *types.Block) *bambooProto.Block {
	ts, _ := ptypes.TimestampProto(t.Timestamp)

	collectionHashes := make([]*bambooProto.SignedCollectionHash, 0)
	for _, hash := range t.SignedCollectionHashes {
		collectionHashes = append(collectionHashes, SignedCollectionHashToMessage(&hash))
	}

	blockSeals := make([]*bambooProto.BlockSeal, 0)
	for _, seal := range t.BlockSeals {
		blockSeals = append(blockSeals, BlockSealToMessage(&seal))
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

func BlockSealToMessage(t *types.BlockSeal) *bambooProto.BlockSeal {
	erSigs := make([][]byte, 0)
	for _, sig := range t.ExecutionReceiptSignatures {
		erSigs = append(erSigs, sig.Bytes())
	}

	raSigs := make([][]byte, 0)
	for _, sig := range t.ResultApprovalSignatures {
		raSigs = append(raSigs, sig.Bytes())
	}

	return &bambooProto.BlockSeal{
		BlockHash:                  t.BlockHash.Bytes(),
		ExecutionReceiptHash:       t.ExecutionReceiptHash.Bytes(),
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

func TransactionToMessage(t *types.Transaction) *bambooProto.Transaction {
	registers := make([]*bambooProto.TransactionRegister, 0)
	for _, r := range t.Registers {
		registers = append(registers, TransactionRegisterToMessage(&r))
	}

	return &bambooProto.Transaction{
		Script:    t.Script,
		Nonce:     t.Nonce,
		Registers: registers,
		Chunks:    t.Chunks,
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

func SignedTransactionToMessage(t *types.SignedTransaction) *bambooProto.SignedTransaction {
	sigs := make([][]byte, 0)
	for _, sig := range t.ScriptSignatures {
		sigs = append(sigs, sig.Bytes())
	}

	return &bambooProto.SignedTransaction{
		Transaction:      TransactionToMessage(&t.Transaction),
		ScriptSignatures: sigs,
		PayerSignature:   t.PayerSignature.Bytes(),
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

func ExecutionReceiptToMessage(t *types.ExecutionReceipt) *bambooProto.ExecutionReceipt {
	registers := make([]*bambooProto.Register, 0)
	for _, r := range t.InitialRegisters {
		registers = append(registers, RegisterToMessage(&r))
	}

	irList := make([]*bambooProto.IntermediateRegisters, 0)
	for _, ir := range t.IntermediateRegistersList {
		irList = append(irList, IntermediateRegistersToMessage(&ir))
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

func InvalidExecutionReceiptChallengeToMessage(t *types.InvalidExecutionReceiptChallenge) *bambooProto.InvalidExecutionReceiptChallenge {
	partTransactions := make([]*bambooProto.IntermediateRegisters, 0)
	for _, ir := range t.PartTransactions {
		partTransactions = append(partTransactions, IntermediateRegistersToMessage(&ir))
	}

	return &bambooProto.InvalidExecutionReceiptChallenge{
		ExecutionReceiptHash:      t.ExecutionReceiptHash.Bytes(),
		ExecutionReceiptSignature: t.ExecutionReceiptSignature.Bytes(),
		PartIndex:                 t.PartIndex,
		PartTransactions:          partTransactions,
		Signature:                 t.Signature.Bytes(),
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

func ResultApprovalToMessage(t *types.ResultApproval) *bambooProto.ResultApproval {
	return &bambooProto.ResultApproval{
		BlockHeight:             t.BlockHeight,
		ExecutionReceiptHash:    t.ExecutionReceiptHash.Bytes(),
		ResultApprovalSignature: t.ResultApprovalSignature.Bytes(),
		Proof:                   t.Proof,
		Signature:               t.Signature.Bytes(),
	}
}
