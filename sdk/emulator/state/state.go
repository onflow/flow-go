package state

import (
	"sync"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	eflow "github.com/dapperlabs/flow-go/sdk/emulator/types"
)

// WorldState represents the current state of the blockchain.
type WorldState struct {
	Blocks            map[string]*eflow.Block
	blocksMutex       sync.RWMutex
	Blockchain        []crypto.Hash
	blockchainMutex   sync.RWMutex
	Transactions      map[string]*flow.Transaction
	transactionsMutex sync.RWMutex
	Registers         flow.Registers
	registersMutex    sync.RWMutex
	LatestState       crypto.Hash
	latestStateMutex  sync.RWMutex
}

// NewWorldState instantiates a new state object with a genesis block.
func NewWorldState() *WorldState {
	blocks := make(map[string]*eflow.Block)
	blockchain := make([]crypto.Hash, 0)
	transactions := make(map[string]*flow.Transaction)
	registers := make(flow.Registers)

	genesis := eflow.GenesisBlock()
	blocks[string(genesis.Hash())] = genesis
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
func (ws *WorldState) GetLatestBlock() *eflow.Block {
	ws.blockchainMutex.RLock()
	defer ws.blockchainMutex.RUnlock()

	currHeight := len(ws.Blockchain)
	blockHash := ws.Blockchain[currHeight-1]

	return ws.GetBlockByHash(blockHash)
}

// GetBlockByHash gets a block by hash.
func (ws *WorldState) GetBlockByHash(hash crypto.Hash) *eflow.Block {
	ws.blocksMutex.RLock()
	defer ws.blocksMutex.RUnlock()

	if block, ok := ws.Blocks[string(hash)]; ok {
		return block
	}

	return nil
}

// GetBlockByNumber gets a block by number.
func (ws *WorldState) GetBlockByNumber(number uint64) *eflow.Block {
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
func (ws *WorldState) GetTransaction(hash crypto.Hash) *flow.Transaction {
	ws.transactionsMutex.RLock()
	defer ws.transactionsMutex.RUnlock()

	if tx, ok := ws.Transactions[string(hash)]; ok {
		return tx
	}

	return nil
}

// ContainsTransaction returns true if the transaction exists in the state, false otherwise.
func (ws *WorldState) ContainsTransaction(hash crypto.Hash) bool {
	_, exists := ws.Transactions[string(hash)]
	return exists
}

// SetRegisters commits a set of registers to the state.
func (ws *WorldState) SetRegisters(registers flow.Registers) {
	ws.registersMutex.Lock()
	defer ws.registersMutex.Unlock()

	ws.Registers.MergeWith(registers)
}

// InsertBlock adds a new block to the blockchain.
func (ws *WorldState) InsertBlock(block *eflow.Block) {
	ws.blockchainMutex.Lock()
	ws.blocksMutex.Lock()
	defer ws.blocksMutex.Unlock()
	defer ws.blockchainMutex.Unlock()

	if _, exists := ws.Blocks[string(block.Hash())]; exists {
		return
	}

	ws.Blocks[string(block.Hash())] = block

	ws.Blockchain = append(ws.Blockchain, block.Hash())

	ws.latestStateMutex.Lock()
	defer ws.latestStateMutex.Unlock()

	ws.LatestState = block.Hash()
}

// InsertTransaction inserts a new transaction into the state.
func (ws *WorldState) InsertTransaction(tx *flow.Transaction) {
	ws.transactionsMutex.Lock()
	defer ws.transactionsMutex.Unlock()

	if _, exists := ws.Transactions[string(tx.Hash)]; exists {
		return
	}

	ws.Transactions[string(tx.Hash)] = tx

	ws.latestStateMutex.Lock()
	defer ws.latestStateMutex.Unlock()

	ws.LatestState = tx.Hash
}

// UpdateTransactionStatus updates the transaction status of an existing transaction.
func (ws *WorldState) UpdateTransactionStatus(h crypto.Hash, status flow.TransactionStatus) {
	tx := ws.GetTransaction(h)
	if tx == nil {
		return
	}

	ws.transactionsMutex.Lock()
	defer ws.transactionsMutex.Unlock()

	tx.Status = status
	ws.Transactions[string(tx.Hash)] = tx
}
