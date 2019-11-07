package store

import (
	"sync"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"

	"github.com/dapperlabs/flow-go/sdk/emulator/state"
)

type memStore struct {
	mu sync.RWMutex
	// Tracks worldstates by block hash
	worldStatesByBlockhash   map[string]*state.WorldState
	worldStatesByBlockNumber map[uint64]*state.WorldState
}

func NewMemStore() Store {
	return memStore{
		mu:                       sync.RWMutex{},
		worldStatesByBlockhash:   make(map[string]*state.WorldState),
		worldStatesByBlockNumber: make(map[uint64]*state.WorldState),
	}
}

func (s memStore) GetBlockByHash(hash crypto.Hash) (flow.Block, error) {
	return flow.Block{}, nil
}

func (s memStore) GetBlockByNumber(blockNumber uint64) (flow.Block, error) {
	return flow.Block{}, nil
}

func (s memStore) GetLatestBlock() (flow.Block, error) {
	return flow.Block{}, nil
}

func (s memStore) InsertBlock(block flow.Block) error {
	return nil
}

func (s memStore) GetTransaction(txHash crypto.Hash) (flow.Transaction, error) {
	return flow.Transaction{}, nil
}

func (s memStore) InsertTransaction(tx flow.Transaction) error {
	return nil
}

func (s memStore) GetRegistersView(blockNumber uint64) flow.RegistersView {
	return flow.RegistersView{}
}

func (s memStore) SetRegisters(blockNumber uint64, registers flow.Registers) error {
	return nil
}
