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
