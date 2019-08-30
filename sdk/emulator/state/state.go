package state

import (
	"sync"

	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/pkg/types"

	"github.com/dapperlabs/bamboo-node/sdk/emulator/execution"
	etypes "github.com/dapperlabs/bamboo-node/sdk/emulator/types"
)

// WorldState represents the current state of the blockchain.
type WorldState struct {
	Blocks            map[string]*etypes.Block
	blocksMutex       sync.RWMutex
	Blockchain        []crypto.Hash
	blockchainMutex   sync.RWMutex
	Transactions      map[string]*types.SignedTransaction
	transactionsMutex sync.RWMutex
	Registers         execution.Registers
	registersMutex    sync.RWMutex
	LatestState       crypto.Hash
	latestStateMutex  sync.RWMutex
}

// NewWorldState instantiates a new state object with a genesis block.
func NewWorldState() *WorldState {
	blocks := make(map[string]*etypes.Block)
	blockchain := make([]crypto.Hash, 0)
	transactions := make(map[string]*types.SignedTransaction)
	registers := make(execution.Registers)

	genesis := etypes.GenesisBlock()
	blocks[string(genesis.Hash().Bytes())] = genesis
	blockchain = append(blockchain, genesis.Hash())

	return &WorldState{
		Blocks:       blocks,
		Blockchain:   blockchain,
		Transactions: transactions,
		Registers:    registers,
		LatestState:  genesis.Hash(),
	}
}

// Hash computes the hash over the contents of World State.
func (ws *WorldState) Hash() crypto.Hash {
	return ws.LatestState
}

// GetLatestBlock gets the most recent block in the blockchain.
func (ws *WorldState) GetLatestBlock() *etypes.Block {
	ws.blockchainMutex.RLock()
	defer ws.blockchainMutex.RUnlock()

	currHeight := len(ws.Blockchain)
	blockHash := ws.Blockchain[currHeight-1]

	return ws.GetBlockByHash(blockHash)
}

// GetBlockByHash gets a block by hash.
func (ws *WorldState) GetBlockByHash(hash crypto.Hash) *etypes.Block {
	ws.blocksMutex.RLock()
	defer ws.blocksMutex.RUnlock()

	if block, ok := ws.Blocks[string(hash.Bytes())]; ok {
		return block
	}

	return nil
}

// GetBlockByNumber gets a block by number.
func (ws *WorldState) GetBlockByNumber(number uint64) *etypes.Block {
	ws.blockchainMutex.RLock()
	defer ws.blockchainMutex.RUnlock()

	currHeight := len(ws.Blockchain)
	if int(number) < currHeight {
		blockHash := ws.Blockchain[number]
		return ws.GetBlockByHash(blockHash)
	}

	return nil
}

// GetTransaction gets a transaction by hash.
func (ws *WorldState) GetTransaction(hash crypto.Hash) *types.SignedTransaction {
	ws.transactionsMutex.RLock()
	defer ws.transactionsMutex.RUnlock()

	if tx, ok := ws.Transactions[string(hash.Bytes())]; ok {
		return tx
	}

	return nil
}

// ContainsTransaction returns true if the transaction exists in the state, false otherwise.
func (ws *WorldState) ContainsTransaction(hash crypto.Hash) bool {
	_, exists := ws.Transactions[string(hash.Bytes())]
	return exists
}

// SetRegisters commits a set of registers to the state.
func (ws *WorldState) SetRegisters(registers execution.Registers) {
	ws.registersMutex.Lock()
	defer ws.registersMutex.Unlock()

	ws.Registers.MergeWith(registers)
}

// InsertBlock adds a new block to the blockchain.
func (ws *WorldState) InsertBlock(block *etypes.Block) {
	ws.blockchainMutex.Lock()
	ws.blocksMutex.Lock()
	defer ws.blocksMutex.Unlock()
	defer ws.blockchainMutex.Unlock()

	if _, exists := ws.Blocks[string(block.Hash().Bytes())]; exists {
		return
	}

	ws.Blocks[string(block.Hash().Bytes())] = block

	ws.Blockchain = append(ws.Blockchain, block.Hash())

	ws.latestStateMutex.Lock()
	defer ws.latestStateMutex.Unlock()

	ws.LatestState = block.Hash()
}

// InsertTransaction inserts a new transaction into the state.
func (ws *WorldState) InsertTransaction(tx *types.SignedTransaction) {
	ws.transactionsMutex.Lock()
	defer ws.transactionsMutex.Unlock()

	if _, exists := ws.Transactions[string(tx.Hash().Bytes())]; exists {
		return
	}

	ws.Transactions[string(tx.Hash().Bytes())] = tx

	ws.latestStateMutex.Lock()
	defer ws.latestStateMutex.Unlock()

	ws.LatestState = tx.Hash()
}

// UpdateTransactionStatus updates the transaction status of an existing transaction.
func (ws *WorldState) UpdateTransactionStatus(h crypto.Hash, status types.TransactionStatus) {
	tx := ws.GetTransaction(h)
	if tx == nil {
		return
	}

	ws.transactionsMutex.Lock()
	defer ws.transactionsMutex.Unlock()

	tx.Status = status
	ws.Transactions[string(tx.Hash().Bytes())] = tx
}
