package core

import (
	"github.com/dapperlabs/bamboo-node/internal/emulator/types"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

type EmulatedBlockchain struct {
	worldStateStore map[crypto.Hash]WorldState
	worldState      WorldState
	blockchain      []*types.Block
	txPool          map[crypto.Hash]*types.Transaction
}

func NewEmulatedBlockchain() *EmulatedBlockchain {
	return &EmulatedBlockchain{
		worldStateStore: make(map[crypto.Hash]WorldState),
		worldState:      WorldState{},
		blockchain:      make([]*types.Block, 0),
		txPool:          make(map[crypto.Hash]*types.SignedTransaction),
	}
}
