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

// GetTransaction gets an existing transaction by hash. First looks in pending txPool,
// then looks in current blockchain state.
func (b *EmulatedBlockchain) GetTransaction(hash crypto.Hash) *types.SignedTransaction {
	if tx, ok := b.txPool[hash]; ok {
		return tx
	}

	return b.pendingWorldState.GetTransaction(hash)
}

// GetAccount gets account information associated with an address identifier.
func (b *EmulatedBlockchain) GetAccount(address crypto.Address) *crypto.Account {
	return b.pendingWorldState.GetAccount(address)
}

// SubmitTransaction sends a transaction to the network that is immediately executed,
// updating the current blockchain state. Note that the resulting state is not finalized until
// CommitBlock() is called. However, the pending blockchain state is indexed for testing purposes.
//
// TODO: pass transaction into runtime to be executed
func (b *EmulatedBlockchain) SubmitTransaction(tx *types.SignedTransaction) {
	if _, exists := b.txPool[tx.Hash()]; exists {
		return
	}
	b.txPool[tx.Hash()] = tx
	b.pendingWorldState.InsertTransaction(tx)
	b.updatePendingWorldStates(tx.Hash())
}

func (b *EmulatedBlockchain) updatePendingWorldStates(txHash crypto.Hash) {
	if _, exists := b.intermediateWorldStates[txHash]; exists {
		return
	}

	bytes := b.pendingWorldState.Encode()
	b.intermediateWorldStates[txHash] = bytes
}

// CommitBlock takes all pending transactions and commits them into a block. Note that
// this clears the pending transaction pool and indexes the committed blockchain state
// for testing purposes.
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
	b.commitWorldState(block.Hash())
}

func (b *EmulatedBlockchain) commitWorldState(blockHash crypto.Hash) {
	if _, exists := b.worldStates[blockHash]; exists {
		return
	}

	bytes := b.pendingWorldState.Encode()
	b.worldStates[blockHash] = bytes
}

// SeekToState rewinds the blockchain state to a previously committed history.
// Note this clears all pending transactions in txPool.
func (b *EmulatedBlockchain) SeekToState(hash crypto.Hash) {
	if bytes, ok := b.worldStates[hash]; ok {
		ws := state.Decode(bytes)
		b.pendingWorldState = ws
		b.txPool = make(map[crypto.Hash]*types.SignedTransaction)
	}
}
