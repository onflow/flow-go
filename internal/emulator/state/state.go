package state

import (
	"sync"

	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/pkg/types"

	etypes "github.com/dapperlabs/bamboo-node/internal/emulator/types"
)

// WorldState represents the current state of the blockchain.
type WorldState struct {
	Accounts          map[crypto.Address]*crypto.Account
	accountsMutex     sync.RWMutex
	Blocks            map[crypto.Hash]*etypes.Block
	blocksMutex       sync.RWMutex
	Blockchain        []crypto.Hash
	blockchainMutex   sync.RWMutex
	Transactions      map[crypto.Hash]*types.SignedTransaction
	transactionsMutex sync.RWMutex
	Registers         map[crypto.Hash][]byte
	registersMutex    sync.RWMutex
	LatestState       crypto.Hash
	latestStateMutex  sync.RWMutex
}

// NewWorldState instantiates a new state object with a genesis block.
func NewWorldState() *WorldState {
	accounts := make(map[crypto.Address]*crypto.Account)
	blocks := make(map[crypto.Hash]*etypes.Block)
	blockchain := make([]crypto.Hash, 0)
	transactions := make(map[crypto.Hash]*types.SignedTransaction)
	registers := make(map[crypto.Hash][]byte)

	genesis := etypes.GenesisBlock()
	blocks[genesis.Hash()] = genesis
	blockchain = append(blockchain, genesis.Hash())

	return &WorldState{
		Accounts:     accounts,
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
	currHeight := len(ws.Blockchain)
	blockHash := ws.Blockchain[currHeight-1]
	ws.blockchainMutex.RUnlock()

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
	currHeight := len(ws.Blockchain)

	if int(height) < currHeight {
		blockHash := ws.Blockchain[height]
		ws.blockchainMutex.RUnlock()
		return ws.GetBlockByHash(blockHash)
	}

	ws.blockchainMutex.RUnlock()

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

// GetAccount gets an account by address.
func (ws *WorldState) GetAccount(address crypto.Address) *crypto.Account {
	ws.accountsMutex.RLock()
	defer ws.accountsMutex.RUnlock()

	if account, ok := ws.Accounts[address]; ok {
		return account
	}

	return nil
}

// GetRegister gets a register by hash.
//
// If the register does not exist, an empty byte slice is returned.
func (ws *WorldState) GetRegister(hash crypto.Hash) []byte {
	ws.registersMutex.RLock()
	defer ws.registersMutex.RUnlock()

	return ws.Registers[hash]
}

// SetRegister sets a register to the given value.
func (ws *WorldState) SetRegister(hash crypto.Hash, value []byte) {
	ws.registersMutex.Lock()
	defer ws.registersMutex.Unlock()

	ws.Registers[hash] = value
}

// SetRegisters commmits a set of registers to the state.
func (ws *WorldState) SetRegisters(registers etypes.Registers) {
	ws.registersMutex.Lock()
	defer ws.registersMutex.Unlock()

	for hash, value := range registers {
		ws.Registers[hash] = value
	}
}

func (ws *WorldState) InsertBlock(block *etypes.Block) {
	ws.blocksMutex.Lock()
	defer ws.blocksMutex.Unlock()
	if _, exists := ws.Blocks[block.Hash()]; exists {
		return
	}

	ws.Blocks[block.Hash()] = block

	ws.blockchainMutex.Lock()
	ws.Blockchain = append(ws.Blockchain, block.Hash())
	ws.blockchainMutex.Unlock()

	ws.latestStateMutex.Lock()
	ws.LatestState = block.Hash()
	ws.latestStateMutex.Unlock()
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

// InsertAccount adds a newly created account into the world state.
func (ws *WorldState) InsertAccount(account *crypto.Account) {
	ws.accountsMutex.Lock()
	defer ws.accountsMutex.Unlock()

	if _, exists := ws.Accounts[account.Address]; exists {
		return
	}

	ws.Accounts[account.Address] = account
}

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
