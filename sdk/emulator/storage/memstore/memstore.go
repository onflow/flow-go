package memstore

import (
	"sync"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/emulator/storage"
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

func (s *Store) BlockByHash(hash crypto.Hash) (types.Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	blockNumber := s.blockHashToNumber[hash.Hex()]
	block, ok := s.blocks[blockNumber]
	if !ok {
		return types.Block{}, storage.ErrNotFound{}
	}

	return block, nil
}

func (s *Store) BlockByNumber(blockNumber uint64) (types.Block, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	block, ok := s.blocks[blockNumber]
	if !ok {
		return types.Block{}, storage.ErrNotFound{}
	}

	return block, nil
}

func (s *Store) LatestBlock() (types.Block, error) {
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

	return s.insertBlock(block)
}

func (s *Store) insertBlock(block types.Block) error {
	s.blocks[block.Number] = block
	if block.Number > s.blockHeight {
		s.blockHeight = block.Number
	}

	return nil
}

func (s *Store) CommitBlock(
	block types.Block,
	transactions []flow.Transaction,
	delta types.LedgerDelta,
	events []flow.Event,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.insertBlock(block)
	if err != nil {
		return err
	}

	for _, tx := range transactions {
		err := s.insertTransaction(tx)
		if err != nil {
			return err
		}
	}

	err = s.insertLedgerDelta(block.Number, delta)
	if err != nil {
		return err
	}

	err = s.insertEvents(block.Number, events)
	if err != nil {
		return err
	}

	return nil
}

func (s *Store) TransactionByHash(txHash crypto.Hash) (flow.Transaction, error) {
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

	return s.insertTransaction(tx)
}

func (s *Store) insertTransaction(tx flow.Transaction) error {
	s.transactions[tx.Hash().Hex()] = tx
	return nil
}

func (s *Store) LedgerViewByNumber(blockNumber uint64) *types.LedgerView {
	return types.NewLedgerView(func(key string) ([]byte, error) {
		s.mu.RLock()
		defer s.mu.RUnlock()

		ledger, ok := s.ledger[blockNumber]
		if !ok {
			return nil, nil
		}

		return ledger[key], nil
	})
}

func (s *Store) InsertLedgerDelta(blockNumber uint64, delta types.LedgerDelta) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.insertLedgerDelta(blockNumber, delta)
}

func (s *Store) insertLedgerDelta(blockNumber uint64, delta types.LedgerDelta) error {
	var oldLedger flow.Ledger

	// use empty ledger if this is the genesis block
	if blockNumber == 0 {
		oldLedger = make(flow.Ledger)
	} else {
		oldLedger = s.ledger[blockNumber-1]
	}

	newLedger := make(flow.Ledger)

	// copy values from the previous ledger
	for key, value := range oldLedger {
		// do not copy deleted values
		if !delta.HasBeenDeleted(key) {
			newLedger[key] = value
		}
	}

	// write all updated values
	for key, value := range delta.Updates() {
		if value != nil {
			newLedger[key] = value
		}
	}

	s.ledger[blockNumber] = newLedger

	return nil
}

func (s *Store) RetrieveEvents(eventType string, startBlock, endBlock uint64) ([]flow.Event, error) {
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

func (s *Store) InsertEvents(blockNumber uint64, events []flow.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.insertEvents(blockNumber, events)
}

func (s *Store) insertEvents(blockNumber uint64, events []flow.Event) error {
	if s.eventsByBlockNumber[blockNumber] == nil {
		s.eventsByBlockNumber[blockNumber] = events
	} else {
		s.eventsByBlockNumber[blockNumber] = append(s.eventsByBlockNumber[blockNumber], events...)
	}

	return nil
}
