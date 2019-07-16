package core

import (
	"time"

	"github.com/dapperlabs/bamboo-node/internal/emulator/state"
	"github.com/dapperlabs/bamboo-node/internal/emulator/types"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

// EmulatedBlockchain simulates a blockchain in the background to enable easy smart contract testing.
// Contains a versioned World State store for granular state update tests.
type EmulatedBlockchain struct {
	worldStates             map[crypto.Hash][]byte
	intermediateWorldStates map[crypto.Hash][]byte
	pendingWorldState       *state.WorldState
	txPool                  map[crypto.Hash]*types.SignedTransaction
}

// NewEmulatedBlockchain instantiates a new blockchain backend for testing purposes.
func NewEmulatedBlockchain() *EmulatedBlockchain {
	return &EmulatedBlockchain{
		worldStates:             make(map[crypto.Hash][]byte),
		intermediateWorldStates: make(map[crypto.Hash][]byte),
		pendingWorldState:       state.NewWorldState(),
		txPool:                  make(map[crypto.Hash]*types.SignedTransaction),
	}
}

func (b *EmulatedBlockchain) SubmitTransaction(tx *types.SignedTransaction) {
	if _, exists := b.txPool[tx.Hash()]; exists {
		return
	}
	b.txPool[tx.Hash()] = tx
	b.pendingWorldState.InsertTransaction(tx)
	b.updatePendingWorldStates()
}

func (b *EmulatedBlockchain) CommitBlock() {
	txHashes := make([]crypto.Hash, 0)
	for hash := range b.txPool {
		txHashes = append(txHashes, hash)
	}
	b.txPool = make(map[crypto.Hash]*types.SignedTransaction)

	prevBlock := b.pendingWorldState.GetLatestBlock()
	block := &types.Block{
		Height:            prevBlock.Height + 1,
		Timestamp:         time.Now(),
		PreviousBlockHash: prevBlock.Hash(),
		TransactionHashes: txHashes,
	}

	b.pendingWorldState.InsertBlock(block)
	b.commitWorldState()
}

func (b *EmulatedBlockchain) GetTransaction(hash crypto.Hash) *types.SignedTransaction {
	if tx, ok := b.txPool[hash]; ok {
		return tx
	}

	return b.pendingWorldState.GetTransaction(hash)
}

func (b *EmulatedBlockchain) GetAccount(address crypto.Address) *crypto.Account {
	return b.pendingWorldState.GetAccount(address)
}

func (b *EmulatedBlockchain) updatePendingWorldStates() {
	bytes := b.pendingWorldState.Encode()
	worldStateHash := crypto.NewHash(bytes)

	if _, exists := b.intermediateWorldStates[worldStateHash]; exists {
		return
	}

	if _, exists := b.worldStates[worldStateHash]; exists {
		return
	}

	b.intermediateWorldStates[worldStateHash] = bytes
}

func (b *EmulatedBlockchain) commitWorldState() {
	bytes := b.pendingWorldState.Encode()
	worldStateHash := crypto.NewHash(bytes)

	if _, exists := b.worldStates[worldStateHash]; exists {
		return
	}

	b.worldStates[worldStateHash] = bytes
}

func (b *EmulatedBlockchain) SeekToState(hash crypto.Hash) {
	if bytes, ok := b.worldStates[hash]; ok {
		ws := state.Decode(bytes)
		b.pendingWorldState = ws
		b.txPool = make(map[crypto.Hash]*types.SignedTransaction)
	}
}
