package storage

import (
	"sync"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/emulator/types"
)

// memStore implements the Store interface with an in-memory store.
type memStore struct {
	mu sync.RWMutex
	// Maps block hashes to block numbers
	blockHashToNumber map[string]uint64
	// Finalized blocks indexed by block number
	blocks []types.Block
	// Finalized transactions by hash
	transactions map[string]flow.Transaction
	// Register states by block number
	registers map[uint64]flow.Registers
	// Stores events by block number
	eventsByBlockNumber map[uint64][]flow.Event
}

// NewMemStore returns a new in-memory Store implementation.
func NewMemStore() Store {
	return memStore{
		mu:                  sync.RWMutex{},
		blockHashToNumber:   make(map[string]uint64),
		blocks:              []types.Block{types.GenesisBlock()},
		transactions:        make(map[string]flow.Transaction),
		registers:           make(map[uint64]flow.Registers),
		eventsByBlockNumber: make(map[uint64][]flow.Event),
	}
}

func (s memStore) GetBlockByHash(hash crypto.Hash) (types.Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	blockNumber := s.blockHashToNumber[hash.Hex()]
	if blockNumber > s.blockHeight() {
		return types.Block{}, ErrNotFound{}
	}
	return s.blocks[int(blockNumber)], nil
}

func (s memStore) GetBlockByNumber(blockNumber uint64) (types.Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if blockNumber > s.blockHeight() {
		return types.Block{}, ErrNotFound{}
	}
	return s.blocks[int(blockNumber)], nil
}

func (s memStore) GetLatestBlock() (types.Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.getLatestBlock(), nil
}

func (s memStore) InsertBlock(block types.Block) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	nextBlockHeight := s.blockHeight() + 1
	if block.Number != nextBlockHeight {
		// TODO: create error type
		return NewInvalidBlockNumberError(nextBlockHeight-1, block.Number)
	}
	s.blocks = append(s.blocks, block)
	return nil
}

func (s memStore) GetTransaction(txHash crypto.Hash) (flow.Transaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tx, ok := s.transactions[txHash.Hex()]
	if !ok {
		return flow.Transaction{}, ErrNotFound{}
	}
	return tx, nil
}

func (s memStore) InsertTransaction(tx flow.Transaction) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.transactions[tx.Hash().Hex()] = tx
	return nil
}

func (s memStore) GetRegistersView(blockNumber uint64) (flow.RegistersView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if blockNumber > s.blockHeight() {
		return flow.RegistersView{}, ErrNotFound{}
	}
	return *s.registers[blockNumber].NewView(), nil
}

func (s memStore) SetRegisters(blockNumber uint64, registers flow.Registers) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.registers[blockNumber] = registers
	return nil
}

func (s memStore) GetEvents(blockNumber uint64, eventType string, startBlock, endBlock uint64) ([]flow.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var events []flow.Event
	// Filter by block number first
	for i := startBlock; i <= endBlock; i++ {
		if s.eventsByBlockNumber[i] != nil {
			// Check for empty type, which indicates no type filtering
			if eventType == "" {
				events = append(events, s.eventsByBlockNumber[i]...)
				continue
			}

			// Otherwise, only add events with matching type
			for _, event := range s.eventsByBlockNumber[i] {
				if event.Type == eventType {
					events = append(events, event)
				}
			}
		}
	}
	return events, nil
}

func (s memStore) InsertEvents(blockNumber uint64, events ...flow.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.eventsByBlockNumber[blockNumber] == nil {
		s.eventsByBlockNumber[blockNumber] = events
	} else {
		s.eventsByBlockNumber[blockNumber] = append(s.eventsByBlockNumber[blockNumber], events...)
	}

	return nil
}

// Returns the block most recently appended to the chain.
func (s memStore) getLatestBlock() types.Block {
	return s.blocks[len(s.blocks)-1]
}

// Returns the current block height, defined as the block number of the latest
// block.
func (s memStore) blockHeight() uint64 {
	return s.getLatestBlock().Number
}
