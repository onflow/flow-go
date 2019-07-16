package core

import (
	"time"

	"github.com/dapperlabs/bamboo-node/language/runtime"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/pkg/types"

	etypes "github.com/dapperlabs/bamboo-node/internal/emulator/types"
)

type EmulatedBlockchain struct {
	worldStateStore map[crypto.Hash]*WorldState
	worldState      *WorldState
	computer        *Computer
	txPool          map[crypto.Hash]*types.SignedTransaction
}

func NewEmulatedBlockchain() *EmulatedBlockchain {
	return &EmulatedBlockchain{
		worldStateStore: make(map[crypto.Hash]*WorldState),
		worldState:      NewWorldState(),
		txPool:          make(map[crypto.Hash]*types.SignedTransaction),
		computer:        NewComputer(runtime.NewInterpreterRuntime()),
	}
}

func (b *EmulatedBlockchain) SubmitTransaction(tx *types.SignedTransaction) error {
	if _, exists := b.txPool[tx.Hash()]; exists {
		return &ErrDuplicateTransaction{TxHash: tx.Hash()}
	}

	if err := b.validateSignature(tx.PayerSignature); err != nil {
		return &ErrInvalidTransactionSignature{TxHash: tx.Hash()}
	}

	b.worldState.InsertTransaction(tx)
	b.txPool[tx.Hash()] = tx

	registers, succeeded := b.computer.ExecuteTransaction(tx, b.worldState.GetRegister)

	if succeeded {
		b.worldState.CommitRegisters(registers)
		b.worldState.UpdateTransactionStatus(tx.Hash(), types.TransactionSealed)
		return nil
	}

	b.worldState.UpdateTransactionStatus(tx.Hash(), types.TransactionReverted)
	return &ErrTransactionReverted{TxHash: tx.Hash()}
}

func (b *EmulatedBlockchain) CommitBlock() {
	txHashes := make([]crypto.Hash, 0)
	for hash := range b.txPool {
		txHashes = append(txHashes, hash)
	}
	b.txPool = make(map[crypto.Hash]*types.SignedTransaction)

	prevBlock := b.worldState.GetLatestBlock()
	block := &etypes.Block{
		Height:            prevBlock.Height + 1,
		Timestamp:         time.Now(),
		PreviousBlockHash: prevBlock.Hash(),
		TransactionHashes: txHashes,
	}

	b.worldState.InsertBlock(block)
}

func (b *EmulatedBlockchain) GetTransaction(hash crypto.Hash) *types.SignedTransaction {
	if tx, ok := b.txPool[hash]; ok {
		return tx
	}

	return b.worldState.GetTransaction(hash)
}

func (b *EmulatedBlockchain) GetAccount(address crypto.Address) *crypto.Account {
	return b.worldState.GetAccount(address)
}

func (b *EmulatedBlockchain) validateSignature(sig crypto.Signature) error {
	// TODO: validate signatures
	return nil
}
