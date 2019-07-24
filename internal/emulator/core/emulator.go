package core

import (
	"time"

	crypto "github.com/dapperlabs/bamboo-node/pkg/crypto/oldcrypto"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime"
	"github.com/dapperlabs/bamboo-node/pkg/types"

	eruntime "github.com/dapperlabs/bamboo-node/internal/emulator/runtime"
	"github.com/dapperlabs/bamboo-node/internal/emulator/state"
	etypes "github.com/dapperlabs/bamboo-node/internal/emulator/types"
)

// EmulatedBlockchain simulates a blockchain in the background to enable easy smart contract testing.
//
// Contains a versioned World State store and a pending transaction pool for granular state update tests.
// Both "committed" and "intermediate" world states are logged for query by version (state hash) utilities,
// but only "committed" world states are enabled for the SeekToState feature.
type EmulatedBlockchain struct {
	// mapping of committed world states (updated after CommitBlock)
	worldStates map[crypto.Hash][]byte
	// mapping of intermediate world states (updated after SubmitTransaction)
	intermediateWorldStates map[crypto.Hash][]byte
	// current world state
	pendingWorldState *state.WorldState
	// pool of pending transactions waiting to be commmitted (already executed)
	txPool   map[crypto.Hash]*types.SignedTransaction
	computer *Computer
}

// NewEmulatedBlockchain instantiates a new blockchain backend for testing purposes.
func NewEmulatedBlockchain() *EmulatedBlockchain {
	worldStates := make(map[crypto.Hash][]byte)
	intermediateWorldStates := make(map[crypto.Hash][]byte)
	txPool := make(map[crypto.Hash]*types.SignedTransaction)
	computer := NewComputer(runtime.NewInterpreterRuntime())
	ws := state.NewWorldState()

	bytes := ws.Encode()
	worldStates[ws.Hash()] = bytes

	return &EmulatedBlockchain{
		worldStates:             worldStates,
		intermediateWorldStates: intermediateWorldStates,
		pendingWorldState:       ws,
		txPool:                  txPool,
		computer:                computer,
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

// GetTransactionAtVersion gets an existing transaction by hash at a specified state.
func (b *EmulatedBlockchain) GetTransactionAtVersion(txHash, version crypto.Hash) (*types.SignedTransaction, error) {
	ws, err := b.getWorldStateAtVersion(version)
	if err != nil {
		return nil, err
	}

	return ws.GetTransaction(txHash), nil
}

// GetAccount gets account information associated with an address identifier.
func (b *EmulatedBlockchain) GetAccount(address crypto.Address) *crypto.Account {
	registers := b.pendingWorldState.Registers.NewView()
	runtimeAPI := eruntime.NewEmulatorRuntimeAPI(registers)
	return runtimeAPI.GetAccount(address)
}

// GetAccountAtVersion gets account information associated with an address identifier at a specified state.
func (b *EmulatedBlockchain) GetAccountAtVersion(address crypto.Address, version crypto.Hash) (*crypto.Account, error) {
	ws, err := b.getWorldStateAtVersion(version)
	if err != nil {
		return nil, err
	}

	registers := ws.Registers.NewView()
	runtimeAPI := eruntime.NewEmulatorRuntimeAPI(registers)
	return runtimeAPI.GetAccount(address), nil
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

	registers := b.pendingWorldState.Registers.NewView()
	err := b.computer.ExecuteTransaction(tx, registers)
	if err != nil {
		b.pendingWorldState.UpdateTransactionStatus(tx.Hash(), types.TransactionReverted)

		b.updatePendingWorldStates(tx.Hash())

		return &ErrTransactionReverted{TxHash: tx.Hash(), Err: err}
	}

	b.pendingWorldState.SetRegisters(registers.UpdatedRegisters())
	b.pendingWorldState.UpdateTransactionStatus(tx.Hash(), types.TransactionFinalized)

	b.updatePendingWorldStates(tx.Hash())

	return nil
}

func (b *EmulatedBlockchain) updatePendingWorldStates(txHash crypto.Hash) {
	if _, exists := b.intermediateWorldStates[txHash]; exists {
		return
	}

	bytes := b.pendingWorldState.Encode()
	b.intermediateWorldStates[txHash] = bytes
}

// CallScript executes a read-only script against the world state and returns the result.
func (b *EmulatedBlockchain) CallScript(script []byte) (interface{}, error) {
	registers := b.pendingWorldState.Registers.NewView()
	return b.computer.ExecuteScript(script, registers)
}

// CallScriptAtVersion executes a read-only script against a specified world state and returns the result.
func (b *EmulatedBlockchain) CallScriptAtVersion(script []byte, version crypto.Hash) (interface{}, error) {
	ws, err := b.getWorldStateAtVersion(version)
	if err != nil {
		return nil, err
	}

	registers := ws.Registers.NewView()
	return b.computer.ExecuteScript(script, registers)
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

// SeekToState rewinds the blockchain state to a previously committed history.
//
// Note that this only seeks to a committed world state (not intermediate world state)
// and this clears all pending transactions in txPool.
func (b *EmulatedBlockchain) SeekToState(hash crypto.Hash) {
	if bytes, ok := b.worldStates[hash]; ok {
		ws := state.Decode(bytes)
		b.pendingWorldState = ws
		b.txPool = make(map[crypto.Hash]*types.SignedTransaction)
	}
}

func (b *EmulatedBlockchain) getWorldStateAtVersion(wsHash crypto.Hash) (*state.WorldState, error) {
	if wsBytes, ok := b.worldStates[wsHash]; ok {
		return state.Decode(wsBytes), nil
	}

	if wsBytes, ok := b.intermediateWorldStates[wsHash]; ok {
		return state.Decode(wsBytes), nil
	}

	return nil, &ErrInvalidStateVersion{Version: wsHash}
}

func (b *EmulatedBlockchain) commitWorldState(blockHash crypto.Hash) {
	if _, exists := b.worldStates[blockHash]; exists {
		return
	}

	bytes := b.pendingWorldState.Encode()
	b.worldStates[blockHash] = bytes
}

func (b *EmulatedBlockchain) validateSignature(sig crypto.Signature) error {
	// TODO: validate signatures
	return nil
}
