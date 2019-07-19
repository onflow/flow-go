package state

import (
	"sync"

	crypto "github.com/dapperlabs/bamboo-node/pkg/crypto/oldcrypto"
	"github.com/dapperlabs/bamboo-node/pkg/types"

	etypes "github.com/dapperlabs/bamboo-node/internal/emulator/types"
)

// WorldState represents the current state of the blockchain.
type WorldState struct {
	Blocks            map[crypto.Hash]*etypes.Block
	blocksMutex       sync.RWMutex
	Blockchain        []crypto.Hash
	blockchainMutex   sync.RWMutex
	Transactions      map[crypto.Hash]*types.SignedTransaction
	transactionsMutex sync.RWMutex
	Registers         etypes.Registers
	registersMutex    sync.RWMutex
	LatestState       crypto.Hash
	latestStateMutex  sync.RWMutex
}

// NewWorldState instantiates a new state object with a genesis block.
func NewWorldState() *WorldState {
	blocks := make(map[crypto.Hash]*etypes.Block)
	blockchain := make([]crypto.Hash, 0)
	transactions := make(map[crypto.Hash]*types.SignedTransaction)
	registers := make(etypes.Registers)

	genesis := etypes.GenesisBlock()
	blocks[genesis.Hash()] = genesis
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
	block := ws.GetBlockByHash(blockHash)
	return block
}

// GetBlockByHash gets a block by hash.
func (ws *WorldState) GetBlockByHash(hash crypto.Hash) *etypes.Block {
	ws.blocksMutex.RLock()
	defer ws.blocksMutex.RUnlock()

	if block, ok := ws.Blocks[hash]; ok {
		return block
	}

	return nil
}

// GetBlockByHeight gets a block by height.
func (ws *WorldState) GetBlockByHeight(height uint64) *etypes.Block {
	ws.blockchainMutex.RLock()
	defer ws.blockchainMutex.RUnlock()

	currHeight := len(ws.Blockchain)
	if int(height) < currHeight {
		blockHash := ws.Blockchain[height]
		return ws.GetBlockByHash(blockHash)
	}

	return nil
}

// GetTransaction gets a transaction by hash.
func (ws *WorldState) GetTransaction(hash crypto.Hash) *types.SignedTransaction {
	ws.transactionsMutex.RLock()
	defer ws.transactionsMutex.RUnlock()

	if tx, ok := ws.Transactions[hash]; ok {
		return tx
	}

	return nil
}

// ContainsTransaction returns true if the transaction exists in the state, false otherwise.
func (ws *WorldState) ContainsTransaction(hash crypto.Hash) bool {
	_, exists := ws.Transactions[hash]
	return exists
}

// SetRegisters commmits a set of registers to the state.
func (ws *WorldState) SetRegisters(registers etypes.Registers) {
	ws.registersMutex.Lock()
	defer ws.registersMutex.Unlock()

	ws.Registers.MergeWith(registers)
}

// InsertBlock adds a new block to the blockchain.
func (ws *WorldState) InsertBlock(block *etypes.Block) {
	ws.blocksMutex.Lock()
	defer ws.blocksMutex.Unlock()

	if _, exists := ws.Blocks[block.Hash()]; exists {
		return
	}

	ws.Blocks[block.Hash()] = block

	ws.blockchainMutex.Lock()
	defer ws.blockchainMutex.Unlock()

	ws.Blockchain = append(ws.Blockchain, block.Hash())

	ws.latestStateMutex.Lock()
	defer ws.latestStateMutex.Unlock()

	ws.LatestState = block.Hash()
}

// InsertTransaction inserts a new transaction into the state.
func (ws *WorldState) InsertTransaction(tx *types.SignedTransaction) {
	ws.transactionsMutex.Lock()
	defer ws.transactionsMutex.Unlock()

	if _, exists := ws.Transactions[tx.Hash()]; exists {
		return
	}

	ws.Transactions[tx.Hash()] = tx

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
	ws.Transactions[tx.Hash()] = tx
}
