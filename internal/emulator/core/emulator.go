package core

import (
	"github.com/dapperlabs/bamboo-node/internal/emulator/types"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

type EmulatedBlockchain struct {
	worldStateStore map[crypto.Hash]*WorldState
	worldState      *WorldState
	txPool          map[crypto.Hash]*types.SignedTransaction
}

func NewEmulatedBlockchain() *EmulatedBlockchain {
	return &EmulatedBlockchain{
		worldStateStore: make(map[crypto.Hash]*WorldState),
		worldState:      NewWorldState(),
		txPool:          make(map[crypto.Hash]*types.SignedTransaction),
	}
}
