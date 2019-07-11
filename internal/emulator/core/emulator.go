package core

import (
	"github.com/dapperlabs/bamboo-node/internal/emulator/types"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

type EmulatedBlockchain struct {
	worldStateStore map[crypto.Hash]*WorldState
	worldState      *WorldState
	blockchain      []*types.Block
	txPool          map[crypto.Hash]*types.SignedTransaction
}

func NewEmulatedBlockchain() *EmulatedBlockchain {
	genesis := types.GenesisBlock()

	worldState := NewWorldState(genesis)
	blockchain := []*types.Block{genesis}

	return &EmulatedBlockchain{
		worldStateStore: make(map[crypto.Hash]*WorldState),
		worldState:      worldState,
		blockchain:      blockchain,
		txPool:          make(map[crypto.Hash]*types.SignedTransaction),
	}
}
