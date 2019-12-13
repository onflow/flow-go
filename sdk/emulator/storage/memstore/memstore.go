package memstore

import (
	"fmt"
	"sync"
	"time"

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
	blocks map[uint64]types.Block
	// Finalized transactions by hash
	transactions map[string]flow.Transaction
	// Ledger states by block number
	ledger map[uint64]flow.Ledger
	// Stores events by block number
	eventsByBlockNumber map[uint64][]flow.Event
	// Tracks the highest block number
	blockHeight uint64
}

// New returns a new in-memory Store implementation.
func New() *Store {
	return &Store{
		mu:                  sync.RWMutex{},
		blockHashToNumber:   make(map[string]uint64),
		blocks:              make(map[uint64]types.Block),
		transactions:        make(map[string]flow.Transaction),
		ledger:              make(map[uint64]flow.Ledger),
		eventsByBlockNumber: make(map[uint64][]flow.Event),
	}
}

func (s *Store) GetBlockByHash(hash crypto.Hash) (types.Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	blockNumber := s.blockHashToNumber[hash.Hex()]
	block, ok := s.blocks[blockNumber]
	if !ok {
		return types.Block{}, storage.ErrNotFound{}
	}

	return block, nil
}

func (s *Store) GetBlockByNumber(blockNumber uint64) (types.Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	block, ok := s.blocks[blockNumber]
	if !ok {
		return types.Block{}, storage.ErrNotFound{}
	}

	return block, nil
}

func (s *Store) GetLatestBlock() (types.Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	latestBlock, ok := s.blocks[s.blockHeight]
	if !ok {
		return types.Block{}, storage.ErrNotFound{}
	}
	return latestBlock, nil
}

func (s *Store) InsertBlock(block types.Block) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.blocks[block.Number] = block
	if block.Number > s.blockHeight {
		s.blockHeight = block.Number
	}

	return nil
}

func (s *Store) CommitPendingBlock(pendingBlock *types.PendingBlock) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, tx := range pendingBlock.Transactions() {
		if err := s.InsertTransaction(*tx); err != nil {
			return err
		}
	}

	if err := s.InsertBlock(*pendingBlock.Header); err != nil {
		return err
	}

	if err := s.SetLedger(pendingBlock.Header.Number, pendingBlock.State); err != nil {
		return err
	}

	if err := s.InsertEvents(pendingBlock.Header.Number, pendingBlock.Events()...); err != nil {
		return err
	}

	return nil
}

func (s *Store) GetTransaction(txHash crypto.Hash) (flow.Transaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tx, ok := s.transactions[txHash.Hex()]
	if !ok {
		return flow.Transaction{}, storage.ErrNotFound{}
	}
	return tx, nil
}

func (s *Store) InsertTransaction(tx flow.Transaction) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.transactions[tx.Hash().Hex()] = tx
	return nil
}

func (s *Store) GetLedger(blockNumber uint64) (flow.Ledger, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ledger, ok := s.ledger[blockNumber]
	if !ok {
		return flow.Ledger{}, storage.ErrNotFound{}
	}
	return ledger, nil
}

func (s *Store) SetLedger(blockNumber uint64, ledger flow.Ledger) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ledger[blockNumber] = ledger
	return nil
}

func (s *Store) GetEvents(eventType string, startBlock, endBlock uint64) ([]flow.Event, error) {
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

func (s *Store) InsertEvents(blockNumber uint64, events ...flow.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.eventsByBlockNumber[blockNumber] == nil {
		s.eventsByBlockNumber[blockNumber] = events
	} else {
		s.eventsByBlockNumber[blockNumber] = append(s.eventsByBlockNumber[blockNumber], events...)
	}

	return nil
}

// Returns the block with the highest number.
func (s *Store) getLatestBlock() types.Block {
	return s.blocks[s.blockHeight]
}
