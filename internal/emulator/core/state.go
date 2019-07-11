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
	transactions      map[crypto.Hash]*types.SignedTransaction
	transactionsMutex sync.RWMutex
}

func NewWorldState(genesis *types.Block) *WorldState {
	blocks := make(map[crypto.Hash]*types.Block)
	blocks[genesis.Hash()] = genesis

	return &WorldState{
		accounts:     make(map[crypto.Address]crypto.Account),
		blocks:       blocks,
		transactions: make(map[crypto.Hash]*types.SignedTransaction),
	}
}
