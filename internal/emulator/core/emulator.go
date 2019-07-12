package core

import (
	"time"

	"github.com/dapperlabs/bamboo-node/internal/emulator/types"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

type EmulatedBlockchain struct {
	worldStateStore map[crypto.Hash]*WorldState
	worldState      *WorldState
	txPool          map[crypto.Hash]*types.SignedTransaction
}

func NewEmulatedBlockchain() *EmulatedBlockchain {
	return &EmulatedBlockchain{
		worldStateStore: make(map[crypto.Hash]*WorldState),
		worldState:      NewWorldState(),
		txPool:          make(map[crypto.Hash]*types.SignedTransaction),
	}
}

func (b *EmulatedBlockchain) SubmitTransaction(tx *types.SignedTransaction) {
	if _, exists := b.txPool[tx.Hash()]; exists {
		return
	}
	b.txPool[tx.Hash()] = tx
}

func (b *EmulatedBlockchain) CommitBlock() {
	txHashes := make([]crypto.Hash, 0)
	for hash, tx := range b.txPool {
		txHashes = append(txHashes, hash)
		b.worldState.InsertTransaction(tx)
	}

	prevBlock := b.worldState.GetLatestBlock()
	block := &types.Block{
		Height:            prevBlock.Height + 1,
		Timestamp:         time.Now(),
		PreviousBlockHash: prevBlock.Hash(),
		TransactionHashes: txHashes,
	}

	b.worldState.InsertBlock(block)
}
