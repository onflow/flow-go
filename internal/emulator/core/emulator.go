package core

import (
	"time"

	"github.com/dapperlabs/bamboo-node/internal/emulator/types"
	"github.com/dapperlabs/bamboo-node/language/runtime"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
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

func (b *EmulatedBlockchain) SubmitTransaction(tx *types.SignedTransaction) {
	if _, exists := b.txPool[tx.Hash()]; exists {
		return
	}

	b.worldState.InsertTransaction(tx)
	b.txPool[tx.Hash()] = tx

	registers, succeeded := b.computer.ExecuteTransaction(tx, b.worldState.GetRegister)

	if succeeded {
		b.worldState.CommitRegisters(registers)
		b.worldState.UpdateTransactionStatus(tx.Hash(), types.TransactionSealed)
	} else {
		b.worldState.UpdateTransactionStatus(tx.Hash(), types.TransactionReverted)
	}
}

func (b *EmulatedBlockchain) CommitBlock() {
	txHashes := make([]crypto.Hash, 0)
	for hash := range b.txPool {
		txHashes = append(txHashes, hash)
	}
	b.txPool = make(map[crypto.Hash]*types.SignedTransaction)

	prevBlock := b.worldState.GetLatestBlock()
	block := &types.Block{
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
