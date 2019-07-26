package messageable

import (
	"github.com/golang/protobuf/ptypes"

	bambooProto "github.com/dapperlabs/bamboo-node/pkg/grpc/shared"

	"github.com/dapperlabs/bamboo-node/internal/pkg/types"
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
	registers := make([]types.Register, len(m.GetRegisters()))
	for i, r := range m.GetRegisters() {
		registers[i] = *MessageToRegister(r)
	}

	return &types.IntermediateRegisters{
		TransactionHash: crypto.BytesToHash(m.GetTransactionHash()),
		Registers:       registers,
		ComputeUsed:     m.GetComputeUsed(),
	}
}

func IntermediateRegistersToMessage(t *types.IntermediateRegisters) *bambooProto.IntermediateRegisters {
	registers := make([]*bambooProto.Register, len(t.Registers))
	for i, r := range t.Registers {
		registers[i] = RegisterToMessage(&r)
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
	keys := make([]types.KeyWeight, len(m.GetKeys()))
	for i, key := range m.GetKeys() {
		keys[i] = *MessageToKeyWeight(key)
	}

	return &types.TransactionRegister{
		Type:       types.Type(m.GetType()),
		AccessMode: types.AccessMode(m.GetAccessMode()),
		ID:         m.GetId(),
		Keys:       keys,
	}
}

func TransactionRegisterToMessage(t *types.TransactionRegister) *bambooProto.TransactionRegister {
	keys := make([]*bambooProto.TransactionRegister_KeyWeight, len(t.Keys))
	for i, key := range t.Keys {
		keys[i] = KeyWeightToMessage(&key)
	}

	return &bambooProto.TransactionRegister{
		Type:       bambooProto.TransactionRegister_Type(t.Type),
		AccessMode: bambooProto.TransactionRegister_AccessMode(t.AccessMode),
		Id:         t.ID,
		Keys:       keys,
	}
}

func MessageToCollection(m *bambooProto.Collection) *types.Collection {
	transactions := make([]types.SignedTransaction, len(m.GetTransactions()))
	for i, tx := range m.GetTransactions() {
		transactions[i] = *MessageToSignedTransaction(tx)
	}

	return &types.Collection{
		Transactions:        transactions,
		FoundationBlockHash: crypto.BytesToHash(m.GetFoundationBlockHash()),
	}
}

func CollectionToMessage(t *types.Collection) *bambooProto.Collection {
	transactions := make([]*bambooProto.SignedTransaction, len(t.Transactions))
	for i, tx := range t.Transactions {
		transactions[i] = SignedTransactionToMessage(&tx)
	}

	return &bambooProto.Collection{
		Transactions:        transactions,
		FoundationBlockHash: t.FoundationBlockHash.Bytes(),
	}
}

func MessageToSignedCollectionHash(m *bambooProto.SignedCollectionHash) *types.SignedCollectionHash {
	sigs := make([]crypto.Signature, len(m.GetSignatures()))
	for i, sig := range m.GetSignatures() {
		sigs[i] = crypto.BytesToSig(sig)
	}

	return &types.SignedCollectionHash{
		CollectionHash: crypto.BytesToHash(m.GetCollectionHash()),
		Signatures:     sigs,
	}
}

func SignedCollectionHashToMessage(t *types.SignedCollectionHash) *bambooProto.SignedCollectionHash {
	sigs := make([][]byte, len(t.Signatures))
	for i, sig := range t.Signatures {
		sigs[i] = sig.Bytes()
	}

	return &bambooProto.SignedCollectionHash{
		CollectionHash: t.CollectionHash.Bytes(),
		Signatures:     sigs,
	}
}

func MessageToBlock(m *bambooProto.Block) *types.Block {
	timestamp, _ := ptypes.Timestamp(m.GetTimestamp())

	collectionHashes := make([]types.SignedCollectionHash, len(m.GetSignedCollectionHashes()))
	for i, hash := range m.GetSignedCollectionHashes() {
		collectionHashes[i] = *MessageToSignedCollectionHash(hash)
	}

	blockSeals := make([]types.BlockSeal, len(m.GetBlockSeals()))
	for i, seal := range m.GetBlockSeals() {
		blockSeals[i] = *MessageToBlockSeal(seal)
	}

	sigs := make([]crypto.Signature, len(m.GetSignatures()))
	for i, sig := range m.GetSignatures() {
		sigs[i] = crypto.BytesToSig(sig)
	}

	return &types.Block{
		ChainID:                m.GetChainID(),
		Height:                 m.GetHeight(),
		PreviousBlockHash:      crypto.BytesToHash(m.GetPreviousBlockHash()),
		Timestamp:              timestamp,
		SignedCollectionHashes: collectionHashes,
		BlockSeals:             blockSeals,
		Signatures:             sigs,
	}
}

func BlockToMessage(t *types.Block) *bambooProto.Block {
	timestamp, _ := ptypes.TimestampProto(t.Timestamp)

	collectionHashes := make([]*bambooProto.SignedCollectionHash, len(t.SignedCollectionHashes))
	for i, hash := range t.SignedCollectionHashes {
		collectionHashes[i] = SignedCollectionHashToMessage(&hash)
	}

	blockSeals := make([]*bambooProto.BlockSeal, len(t.BlockSeals))
	for i, seal := range t.BlockSeals {
		blockSeals[i] = BlockSealToMessage(&seal)
	}

	sigs := make([][]byte, len(t.Signatures))
	for i, sig := range t.Signatures {
		sigs[i] = sig.Bytes()
	}

	return &bambooProto.Block{
		ChainID:                t.ChainID,
		Height:                 t.Height,
		PreviousBlockHash:      t.PreviousBlockHash.Bytes(),
		Timestamp:              timestamp,
		SignedCollectionHashes: collectionHashes,
		BlockSeals:             blockSeals,
		Signatures:             sigs,
	}
}

func MessageToBlockSeal(m *bambooProto.BlockSeal) *types.BlockSeal {
	erSigs := make([]crypto.Signature, len(m.GetExecutionReceiptSignatures()))
	for i, sig := range m.GetExecutionReceiptSignatures() {
		erSigs[i] = crypto.BytesToSig(sig)
	}

	raSigs := make([]crypto.Signature, len(m.GetResultApprovalSignatures()))
	for i, sig := range m.GetResultApprovalSignatures() {
		raSigs[i] = crypto.BytesToSig(sig)
	}

	return &types.BlockSeal{
		BlockHash:                  crypto.BytesToHash(m.GetBlockHash()),
		ExecutionReceiptHash:       crypto.BytesToHash(m.GetExecutionReceiptHash()),
		ExecutionReceiptSignatures: erSigs,
		ResultApprovalSignatures:   raSigs,
	}
}

func BlockSealToMessage(t *types.BlockSeal) *bambooProto.BlockSeal {
	erSigs := make([][]byte, len(t.ExecutionReceiptSignatures))
	for i, sig := range t.ExecutionReceiptSignatures {
		erSigs[i] = sig.Bytes()
	}

	raSigs := make([][]byte, len(t.ResultApprovalSignatures))
	for i, sig := range t.ResultApprovalSignatures {
		raSigs[i] = sig.Bytes()
	}

	return &bambooProto.BlockSeal{
		BlockHash:                  t.BlockHash.Bytes(),
		ExecutionReceiptHash:       t.ExecutionReceiptHash.Bytes(),
		ExecutionReceiptSignatures: erSigs,
		ResultApprovalSignatures:   raSigs,
	}
}

func MessageToTransaction(m *bambooProto.Transaction) *types.Transaction {
	registers := make([]types.TransactionRegister, len(m.GetRegisters()))
	for i, r := range m.GetRegisters() {
		registers[i] = *MessageToTransactionRegister(r)
	}

	return &types.Transaction{
		Script:    m.GetScript(),
		Nonce:     m.GetNonce(),
		Registers: registers,
		Chunks:    m.GetChunks(),
	}
}

func TransactionToMessage(t *types.Transaction) *bambooProto.Transaction {
	registers := make([]*bambooProto.TransactionRegister, len(t.Registers))
	for i, r := range t.Registers {
		registers[i] = TransactionRegisterToMessage(&r)
	}

	return &bambooProto.Transaction{
		Script:    t.Script,
		Nonce:     t.Nonce,
		Registers: registers,
		Chunks:    t.Chunks,
	}
}

func MessageToSignedTransaction(m *bambooProto.SignedTransaction) *types.SignedTransaction {
	sigs := make([]crypto.Signature, len(m.GetScriptSignatures()))
	for i, sig := range m.GetScriptSignatures() {
		sigs[i] = crypto.BytesToSig(sig)
	}

	return &types.SignedTransaction{
		Transaction:      *MessageToTransaction(m.GetTransaction()),
		ScriptSignatures: sigs,
		PayerSignature:   crypto.BytesToSig(m.GetPayerSignature()),
	}
}

func SignedTransactionToMessage(t *types.SignedTransaction) *bambooProto.SignedTransaction {
	sigs := make([][]byte, len(t.ScriptSignatures))
	for i, sig := range t.ScriptSignatures {
		sigs[i] = sig.Bytes()
	}

	return &bambooProto.SignedTransaction{
		Transaction:      TransactionToMessage(&t.Transaction),
		ScriptSignatures: sigs,
		PayerSignature:   t.PayerSignature.Bytes(),
	}
}

func MessageToExecutionReceipt(m *bambooProto.ExecutionReceipt) *types.ExecutionReceipt {
	registers := make([]types.Register, len(m.GetInitialRegisters()))
	for i, r := range m.GetInitialRegisters() {
		registers[i] = *MessageToRegister(r)
	}

	irList := make([]types.IntermediateRegisters, len(m.GetIntermediateRegistersList()))
	for i, ir := range m.GetIntermediateRegistersList() {
		irList[i] = *MessageToIntermediateRegisters(ir)
	}

	sigs := make([]crypto.Signature, len(m.GetSignatures()))
	for i, sig := range m.GetSignatures() {
		sigs[i] = crypto.BytesToSig(sig)
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
	registers := make([]*bambooProto.Register, len(t.InitialRegisters))
	for i, r := range t.InitialRegisters {
		registers[i] = RegisterToMessage(&r)
	}

	irList := make([]*bambooProto.IntermediateRegisters, len(t.IntermediateRegistersList))
	for i, ir := range t.IntermediateRegistersList {
		irList[i] = IntermediateRegistersToMessage(&ir)
	}

	sigs := make([][]byte, len(t.Signatures))
	for i, sig := range t.Signatures {
		sigs[i] = sig.Bytes()
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
	partTransactions := make([]types.IntermediateRegisters, len(m.GetPartTransactions()))
	for i, ir := range m.GetPartTransactions() {
		partTransactions[i] = *MessageToIntermediateRegisters(ir)
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
	partTransactions := make([]*bambooProto.IntermediateRegisters, len(t.PartTransactions))
	for i, ir := range t.PartTransactions {
		partTransactions[i] = IntermediateRegistersToMessage(&ir)
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

func MessageToStateTransition(m *bambooProto.StateTransition) *types.StateTransition {
	sigs := make([]crypto.Signature, len(m.GetPreviousCommitApprovalSignatures()))
	for i, sig := range m.GetPreviousCommitApprovalSignatures() {
		sigs[i] = crypto.BytesToSig(sig)
	}

	return &types.StateTransition{
		PreviousStateTransitionHash:      crypto.BytesToHash(m.GetPreviousStateTransitionHash()),
		PreviousCommitApprovalSignatures: sigs,
		Height:                           m.GetHeight(),
		Value:                            m.GetValue(),
	}
}

func StateTransitionToMessage(t *types.StateTransition) *bambooProto.StateTransition {
	sigs := make([][]byte, len(t.PreviousCommitApprovalSignatures))
	for i, sig := range t.PreviousCommitApprovalSignatures {
		sigs[i] = sig.Bytes()
	}

	return &bambooProto.StateTransition{
		PreviousStateTransitionHash:      t.PreviousStateTransitionHash.Bytes(),
		PreviousCommitApprovalSignatures: sigs,
		Height:                           t.Height,
		Value:                            t.Value,
	}
}

func MessageToSignedStateTransition(m *bambooProto.SignedStateTransition) *types.SignedStateTransition {
	return &types.SignedStateTransition{
		StateTransition: *MessageToStateTransition(m.GetStateTransition()),
		Signature:       crypto.BytesToSig(m.GetSignature()),
	}
}

func SignedStateTransitionToMessage(t *types.SignedStateTransition) *bambooProto.SignedStateTransition {
	return &bambooProto.SignedStateTransition{
		StateTransition: StateTransitionToMessage(&t.StateTransition),
		Signature:       t.Signature.Bytes(),
	}
}

func MessageToFinalizedStateTransition(m *bambooProto.FinalizedStateTransition) *types.FinalizedStateTransition {
	sigs := make([]crypto.Signature, len(m.GetSignatures()))
	for i, sig := range m.GetSignatures() {
		sigs[i] = crypto.BytesToSig(sig)
	}

	return &types.FinalizedStateTransition{
		SignedStateTransition: *MessageToSignedStateTransition(m.GetSignedStateTransition()),
		Signatures:            sigs,
	}
}

func FinalizedStateTransitionToMessage(t *types.FinalizedStateTransition) *bambooProto.FinalizedStateTransition {
	sigs := make([][]byte, len(t.Signatures))
	for i, sig := range t.Signatures {
		sigs[i] = sig.Bytes()
	}

	return &bambooProto.FinalizedStateTransition{
		SignedStateTransition: SignedStateTransitionToMessage(&t.SignedStateTransition),
		Signatures:            sigs,
	}
}

func MessageToStateTransitionVote(m *bambooProto.StateTransitionVote) *types.StateTransitionVote {
	return &types.StateTransitionVote{
		StateTransitionHash: crypto.BytesToHash(m.GetStateTransitionHash()),
		Vote:                types.Vote(m.GetVote()),
		Height:              m.GetHeight(),
	}
}

func StateTransitionVoteToMessage(t *types.StateTransitionVote) *bambooProto.StateTransitionVote {
	return &bambooProto.StateTransitionVote{
		StateTransitionHash: t.StateTransitionHash.Bytes(),
		Vote:                bambooProto.Vote(t.Vote),
		Height:              t.Height,
	}
}
