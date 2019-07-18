package core

import (
	"time"

	"github.com/dapperlabs/bamboo-node/language/runtime"
	"github.com/dapperlabs/bamboo-node/pkg/types"

	"github.com/dapperlabs/bamboo-node/internal/emulator/state"
	etypes "github.com/dapperlabs/bamboo-node/internal/emulator/types"
	crypto "github.com/dapperlabs/bamboo-node/pkg/crypto/oldcrypto"
)

// EmulatedBlockchain simulates a blockchain in the background to enable easy smart contract testing.
//
// Contains a versioned World State store and a pending transaction pool for granular state update tests.
type EmulatedBlockchain struct {
	worldStates             map[crypto.Hash][]byte
	intermediateWorldStates map[crypto.Hash][]byte
	pendingWorldState       *state.WorldState
	txPool                  map[crypto.Hash]*types.SignedTransaction
	computer                *Computer
}

// NewEmulatedBlockchain instantiates a new blockchain backend for testing purposes.
func NewEmulatedBlockchain() *EmulatedBlockchain {
	return &EmulatedBlockchain{
		worldStates:             make(map[crypto.Hash][]byte),
		intermediateWorldStates: make(map[crypto.Hash][]byte),
		pendingWorldState:       state.NewWorldState(),
		txPool:                  make(map[crypto.Hash]*types.SignedTransaction),
		computer:                NewComputer(runtime.NewInterpreterRuntime()),
	}
}

// GetTransaction gets an existing transaction by hash.
//
// First looks in pending txPool, then looks in current blockchain state.
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

// SubmitTransaction sends a transaction to the network that is immediately executed (updates blockchain state).
//
// Note that the resulting state is not finalized until CommitBlock() is called.
// However, the pending blockchain state is indexed for testing purposes.
func (b *EmulatedBlockchain) SubmitTransaction(tx *types.SignedTransaction) error {
	if _, exists := b.txPool[tx.Hash()]; exists {
		return &ErrDuplicateTransaction{TxHash: tx.Hash()}
	}

	if b.pendingWorldState.ContainsTransaction(tx.Hash()) {
		return &ErrDuplicateTransaction{TxHash: tx.Hash()}
	}

	if err := b.validateSignature(tx.PayerSignature); err != nil {
		return &ErrInvalidTransactionSignature{TxHash: tx.Hash()}
	}

	b.txPool[tx.Hash()] = tx
	b.pendingWorldState.InsertTransaction(tx)

	registers, err := b.computer.ExecuteTransaction(tx, b.pendingWorldState.GetRegister)
	if err != nil {
		b.pendingWorldState.UpdateTransactionStatus(tx.Hash(), types.TransactionReverted)

		b.updatePendingWorldStates(tx.Hash())

		return &ErrTransactionReverted{TxHash: tx.Hash(), Err: err}
	}

	b.pendingWorldState.SetRegisters(registers)
	b.pendingWorldState.UpdateTransactionStatus(tx.Hash(), types.TransactionFinalized)

	b.updatePendingWorldStates(tx.Hash())

	return nil
}

// CallScript executes a read-only script against the world state and returns the result.
func (b *EmulatedBlockchain) CallScript(script []byte) (interface{}, error) {
	return b.computer.ExecuteCall(script, b.pendingWorldState.GetRegister)
}

func (b *EmulatedBlockchain) updatePendingWorldStates(txHash crypto.Hash) {
	if _, exists := b.intermediateWorldStates[txHash]; exists {
		return
	}

	bytes := b.pendingWorldState.Encode()
	b.intermediateWorldStates[txHash] = bytes
}

// CommitBlock takes all pending transactions and commits them into a block.
//
// Note that this clears the pending transaction pool and indexes the committed
// blockchain state for testing purposes.
func (b *EmulatedBlockchain) CommitBlock() {
	txHashes := make([]crypto.Hash, 0)
	for hash := range b.txPool {
		txHashes = append(txHashes, hash)
		if b.pendingWorldState.GetTransaction(hash).Status != types.TransactionReverted {
			b.pendingWorldState.UpdateTransactionStatus(hash, types.TransactionSealed)
		}
	}
	b.txPool = make(map[crypto.Hash]*types.SignedTransaction)

	prevBlock := b.pendingWorldState.GetLatestBlock()
	block := &etypes.Block{
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
//
// Note this clears all pending transactions in txPool.
func (b *EmulatedBlockchain) SeekToState(hash crypto.Hash) {
	if bytes, ok := b.worldStates[hash]; ok {
		ws := state.Decode(bytes)
		b.pendingWorldState = ws
		b.txPool = make(map[crypto.Hash]*types.SignedTransaction)
	}
}

func (b *EmulatedBlockchain) validateSignature(sig crypto.Signature) error {
	// TODO: validate signatures
	return nil
}
