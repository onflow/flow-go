package core

import (
	"sync"

	"github.com/dapperlabs/bamboo-node/internal/emulator/types"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

type WorldState struct {
	accounts          map[crypto.Address]crypto.Account
	accountsMutex     sync.RWMutex
	blocks            map[crypto.Hash]*types.Block
	blocksMutex       sync.RWMutex
	blockchain        []crypto.Hash
	blockchainMutex   sync.RWMutex
	transactions      map[crypto.Hash]*types.SignedTransaction
	transactionsMutex sync.RWMutex
}

func NewWorldState() *WorldState {
	genesis := types.GenesisBlock()
	blocks := make(map[crypto.Hash]*types.Block)
	blocks[genesis.Hash()] = genesis

	return &WorldState{
		accounts:     make(map[crypto.Address]crypto.Account),
		blocks:       blocks,
		blockchain:   []crypto.Hash{genesis.Hash()},
		transactions: make(map[crypto.Hash]*types.SignedTransaction),
	}
}

func (ws *WorldState) GetLatestBlock() *types.Block {
	ws.blockchainMutex.RLock()
	currHeight := len(ws.blockchain)
	blockHash := ws.blockchain[currHeight-1]
	ws.blockchainMutex.RUnlock()

	block := ws.GetBlockByHash(blockHash)
	return block
}

func (ws *WorldState) GetBlockByHash(hash crypto.Hash) *types.Block {
	ws.blocksMutex.RLock()
	defer ws.blocksMutex.RUnlock()

	if block, ok := ws.blocks[hash]; ok {
		return block
	}

	return nil
}

func (ws *WorldState) GetBlockByHeight(height uint64) *types.Block {
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
