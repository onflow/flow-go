package core

import (
	"time"

	"github.com/dapperlabs/bamboo-node/internal/emulator/state"
	"github.com/dapperlabs/bamboo-node/internal/emulator/types"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

type EmulatedBlockchain struct {
	worldStateStore map[crypto.Hash][]byte
	worldState      *state.WorldState
	txPool          map[crypto.Hash]*types.SignedTransaction
}

func NewEmulatedBlockchain() *EmulatedBlockchain {
	return &EmulatedBlockchain{
		worldStateStore: make(map[crypto.Hash][]byte),
		worldState:      state.NewWorldState(),
		txPool:          make(map[crypto.Hash]*types.SignedTransaction),
	}
}

func (b *EmulatedBlockchain) SubmitTransaction(tx *types.SignedTransaction) {
	if _, exists := b.txPool[tx.Hash()]; exists {
		return
	}
	b.txPool[tx.Hash()] = tx
	b.worldState.InsertTransaction(tx)
	b.updateWorldStateStore()
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
	b.updateWorldStateStore()
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

func (b *EmulatedBlockchain) updateWorldStateStore() {
	bytes := b.worldState.Encode()
	worldStateHash := crypto.NewHash(bytes)

	if _, exists := b.worldStateStore[worldStateHash]; exists {
		return
	}

	b.worldStateStore[worldStateHash] = bytes
}
