package core

import (
	"sync"

	etypes "github.com/dapperlabs/bamboo-node/internal/emulator/types"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/pkg/types"
)

type WorldState struct {
	accounts          map[crypto.Address]*crypto.Account
	accountsMutex     sync.RWMutex
	blocks            map[crypto.Hash]*etypes.Block
	blocksMutex       sync.RWMutex
	blockchain        []crypto.Hash
	blockchainMutex   sync.RWMutex
	transactions      map[crypto.Hash]*types.SignedTransaction
	transactionsMutex sync.RWMutex
	registers         map[crypto.Hash][]byte
	registersMutex    sync.RWMutex
}

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
		accounts:     accounts,
		blocks:       blocks,
		blockchain:   blockchain,
		transactions: transactions,
		registers:    registers,
	}
}

func (ws *WorldState) GetLatestBlock() *etypes.Block {
	ws.blockchainMutex.RLock()
	currHeight := len(ws.blockchain)
	blockHash := ws.blockchain[currHeight-1]
	ws.blockchainMutex.RUnlock()

	block := ws.GetBlockByHash(blockHash)
	return block
}

func (ws *WorldState) GetBlockByHash(hash crypto.Hash) *etypes.Block {
	ws.blocksMutex.RLock()
	defer ws.blocksMutex.RUnlock()

	if block, ok := ws.blocks[hash]; ok {
		return block
	}

	return nil
}

func (ws *WorldState) GetBlockByHeight(height uint64) *etypes.Block {
	ws.blockchainMutex.RLock()
	currHeight := len(ws.blockchain)

	if int(height) < currHeight {
		blockHash := ws.blockchain[height]
		ws.blockchainMutex.RUnlock()
		return ws.GetBlockByHash(blockHash)
	}

	ws.blockchainMutex.RUnlock()

	return nil
}

func (ws *WorldState) GetTransaction(hash crypto.Hash) *types.SignedTransaction {
	ws.transactionsMutex.RLock()
	defer ws.transactionsMutex.RUnlock()

	if tx, ok := ws.transactions[hash]; ok {
		return tx
	}

	return nil
}

func (ws *WorldState) GetAccount(address crypto.Address) *crypto.Account {
	ws.accountsMutex.RLock()
	defer ws.accountsMutex.RUnlock()

	if account, ok := ws.accounts[address]; ok {
		return account
	}

	return nil
}

func (ws *WorldState) GetRegister(hash crypto.Hash) []byte {
	ws.registersMutex.RLock()
	defer ws.registersMutex.RUnlock()

	return ws.registers[hash]
}

func (ws *WorldState) SetRegister(hash crypto.Hash, value []byte) {
	ws.registersMutex.Lock()
	defer ws.registersMutex.Unlock()

	ws.registers[hash] = value
}

func (ws *WorldState) CommitRegisters(registers etypes.Registers) {
	ws.registersMutex.Lock()

	for hash, value := range registers {
		ws.registers[hash] = value
	}

	ws.registersMutex.Unlock()
}

func (ws *WorldState) InsertBlock(block *etypes.Block) {
	ws.blocksMutex.Lock()
	defer ws.blocksMutex.Unlock()
	if _, exists := ws.blocks[block.Hash()]; exists {
		return
	}

	ws.blocks[block.Hash()] = block

	ws.blockchainMutex.Lock()
	ws.blockchain = append(ws.blockchain, block.Hash())
	ws.blockchainMutex.Unlock()
}

func (ws *WorldState) InsertTransaction(tx *types.SignedTransaction) {
	ws.transactionsMutex.Lock()
	defer ws.transactionsMutex.Unlock()

	if _, exists := ws.transactions[tx.Hash()]; exists {
		return
	}

	ws.transactions[tx.Hash()] = tx
}

func (ws *WorldState) InsertAccount(account *crypto.Account) {
	ws.accountsMutex.Lock()
	defer ws.accountsMutex.Unlock()

	if _, exists := ws.accounts[account.Address]; exists {
		return
	}

	ws.accounts[account.Address] = account
}

func (ws *WorldState) UpdateTransactionStatus(h crypto.Hash, status types.TransactionStatus) {
	tx := ws.GetTransaction(h)
	if tx == nil {
		return
	}

	ws.transactionsMutex.Lock()
	tx.Status = status
	ws.transactions[tx.Hash()] = tx
	ws.transactionsMutex.Unlock()
}
