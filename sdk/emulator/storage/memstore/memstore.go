package memstore

import (
	"errors"
	"sync"

	"github.com/dapperlabs/flow-go/sdk/emulator/storage"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/emulator/types"
)

// Store implements the Store interface with an in-memory store.
type Store struct {
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

var (
	// ErrInvalidBlockInsertion occurs when a block with a block number that
	// is NOT one greater than the last block number is inserted.
	ErrInvalidBlockInsertion = errors.New("cannot insert block with invalid number")
)

// New returns a new in-memory Store implementation.
func New() Store {
	return Store{
		mu:                  sync.RWMutex{},
		blockHashToNumber:   make(map[string]uint64),
		blocks:              []types.Block{types.GenesisBlock()},
		transactions:        make(map[string]flow.Transaction),
		registers:           make(map[uint64]flow.Registers),
		eventsByBlockNumber: make(map[uint64][]flow.Event),
	}
}

func (s Store) GetBlockByHash(hash crypto.Hash) (types.Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	blockNumber := s.blockHashToNumber[hash.Hex()]
	if blockNumber > s.blockHeight() {
		return types.Block{}, storage.ErrNotFound{}
	}
	return s.blocks[int(blockNumber)], nil
}

func (s Store) GetBlockByNumber(blockNumber uint64) (types.Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if blockNumber > s.blockHeight() {
		return types.Block{}, storage.ErrNotFound{}
	}
	return s.blocks[int(blockNumber)], nil
}

func (s Store) GetLatestBlock() (types.Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.getLatestBlock(), nil
}

func (s Store) InsertBlock(block types.Block) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	nextBlockHeight := s.blockHeight() + 1
	if block.Number != nextBlockHeight {
		return ErrInvalidBlockInsertion
	}
	s.blocks = append(s.blocks, block)
	return nil
}

func (s Store) GetTransaction(txHash crypto.Hash) (flow.Transaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tx, ok := s.transactions[txHash.Hex()]
	if !ok {
		return flow.Transaction{}, storage.ErrNotFound{}
	}
	return tx, nil
}

func (s Store) InsertTransaction(tx flow.Transaction) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.transactions[tx.Hash().Hex()] = tx
	return nil
}

func (s Store) GetRegistersView(blockNumber uint64) (flow.RegistersView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if blockNumber > s.blockHeight() {
		return flow.RegistersView{}, storage.ErrNotFound{}
	}
	return *s.registers[blockNumber].NewView(), nil
}

func (s Store) SetRegisters(blockNumber uint64, registers flow.Registers) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.registers[blockNumber] = registers
	return nil
}

func (s Store) GetEvents(eventType string, startBlock, endBlock uint64) ([]flow.Event, error) {
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

func (s Store) InsertEvents(blockNumber uint64, events ...flow.Event) error {
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
func (s Store) getLatestBlock() types.Block {
	return s.blocks[len(s.blocks)-1]
}

// Returns the current block height, defined as the block number of the latest
// block.
func (s Store) blockHeight() uint64 {
	return s.getLatestBlock().Number
}
