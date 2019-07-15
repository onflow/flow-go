package state

import (
	"sync"

	"github.com/dapperlabs/bamboo-node/internal/emulator/types"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

type WorldState struct {
	Accounts          map[crypto.Address]*crypto.Account
	accountsMutex     sync.RWMutex
	Blocks            map[crypto.Hash]*types.Block
	blocksMutex       sync.RWMutex
	Blockchain        []crypto.Hash
	blockchainMutex   sync.RWMutex
	Transactions      map[crypto.Hash]*types.SignedTransaction
	transactionsMutex sync.RWMutex
}

func NewWorldState() *WorldState {
	accounts := make(map[crypto.Address]*crypto.Account)
	blocks := make(map[crypto.Hash]*types.Block)
	blockchain := make([]crypto.Hash, 0)
	transactions := make(map[crypto.Hash]*types.SignedTransaction)

	genesis := types.GenesisBlock()
	blocks[genesis.Hash()] = genesis
	blockchain = append(blockchain, genesis.Hash())

	return &WorldState{
		Accounts:     accounts,
		Blocks:       blocks,
		Blockchain:   blockchain,
		Transactions: transactions,
	}
}

func (ws *WorldState) Hash() crypto.Hash {
	bytes := ws.Encode()
	return crypto.NewHash(bytes)
}

func (ws *WorldState) GetLatestBlock() *types.Block {
	ws.blockchainMutex.RLock()
	currHeight := len(ws.Blockchain)
	blockHash := ws.Blockchain[currHeight-1]
	ws.blockchainMutex.RUnlock()

	block := ws.GetBlockByHash(blockHash)
	return block
}

func (ws *WorldState) GetBlockByHash(hash crypto.Hash) *types.Block {
	ws.blocksMutex.RLock()
	defer ws.blocksMutex.RUnlock()

	if block, ok := ws.Blocks[hash]; ok {
		return block
	}

	return nil
}

func (ws *WorldState) GetBlockByHeight(height uint64) *types.Block {
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

func (ws *WorldState) GetTransaction(hash crypto.Hash) *types.SignedTransaction {
	ws.transactionsMutex.RLock()
	defer ws.transactionsMutex.RUnlock()

	if tx, ok := ws.Transactions[hash]; ok {
		return tx
	}

	return nil
}

func (ws *WorldState) GetAccount(address crypto.Address) *crypto.Account {
	ws.accountsMutex.RLock()
	defer ws.accountsMutex.RUnlock()

	if account, ok := ws.Accounts[address]; ok {
		return account
	}

	return nil
}

func (ws *WorldState) InsertBlock(block *types.Block) {
	ws.blocksMutex.Lock()
	defer ws.blocksMutex.Unlock()
	if _, exists := ws.Blocks[block.Hash()]; exists {
		return
	}

	ws.Blocks[block.Hash()] = block

	ws.blockchainMutex.Lock()
	ws.Blockchain = append(ws.Blockchain, block.Hash())
	ws.blockchainMutex.Unlock()
}

func (ws *WorldState) InsertTransaction(tx *types.SignedTransaction) {
	ws.transactionsMutex.Lock()
	defer ws.transactionsMutex.Unlock()

	if _, exists := ws.Transactions[tx.Hash()]; exists {
		return
	}

	ws.Transactions[tx.Hash()] = tx
}

func (ws *WorldState) InsertAccount(account *crypto.Account) {
	ws.accountsMutex.Lock()
	defer ws.accountsMutex.Unlock()

	if _, exists := ws.Accounts[account.Address]; exists {
		return
	}

	ws.Accounts[account.Address] = account
}
