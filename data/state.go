package data

import (
	"github.com/dapperlabs/bamboo-emulator/crypto"
)

// WorldState represents the current state of the blockchain.
type WorldState struct {
	Blocks       map[crypto.Hash]Block
	Collections  map[crypto.Hash]Collection
	Transactions map[crypto.Hash]Transaction
	Registers    map[string][]byte
	Blockchain   []Block
}

// NewWorldState returns a new empty world state.
func NewWorldState() *WorldState {
	return &WorldState{
		Blocks:       make(map[crypto.Hash]Block),
		Collections:  make(map[crypto.Hash]Collection),
		Transactions: make(map[crypto.Hash]Transaction),
		Registers:    make(map[string][]byte),
		Blockchain:   make([]Block, 0),
	}
}

// GetLatestBlock returns the latest block in the blockchain.
func (s *WorldState) GetLatestBlock() *Block {
	currHeight := len(s.Blockchain)
	return &s.Blockchain[currHeight-1]
}

// GetBlockByNumber gets a block by number.
func (s *WorldState) GetBlockByNumber(n uint64) (*Block, error) {
	currHeight := len(s.Blockchain)
	if int(n) < currHeight {
		return &s.Blockchain[n], nil
	}

	return nil, &InvalidBlockNumberError{blockNumber: n}
}

// GetBlockByHash gets a block by hash.
func (s *WorldState) GetBlockByHash(h crypto.Hash) (*Block, error) {
	if block, ok := s.Blocks[h]; ok {
		return &block, nil
	}

	return nil, &ItemNotFoundError{hash: h}
}

// GetCollection gets a collection by hash.
func (s *WorldState) GetCollection(h crypto.Hash) (*Collection, error) {
	if collection, ok := s.Collections[h]; ok {
		return &collection, nil
	}

	return nil, &ItemNotFoundError{hash: h}
}

// GetTransaction gets a transaction by hash.
func (s *WorldState) GetTransaction(h crypto.Hash) (*Transaction, error) {
	if tx, ok := s.Transactions[h]; ok {
		return &tx, nil
	}

	return nil, &ItemNotFoundError{hash: h}
}

// GetRegister gets a register by ID.
func (s *WorldState) GetRegister(id string) []byte {
	return s.Registers[id]
}

// AddBlock adds a new block to the blockchain.
func (s *WorldState) AddBlock(block *Block) error {
	// TODO: add to block map and chain
	return nil
}

// InsertCollection inserts a new collection into the state.
func (s *WorldState) InsertCollection(col *Collection) error {
	// TODO: add to collection map
	return nil
}

// InsertTransaction inserts a new transaction into the state.
func (s *WorldState) InsertTransaction(tx *Transaction) error {
	if _, exists := s.Transactions[tx.Hash()]; exists {
		return &DuplicateItemError{hash: tx.Hash()}
	}

	s.Transactions[tx.Hash()] = *tx

	return nil
}

// CommitRegisters updates the register state with the values of a register map.
func (s *WorldState) CommitRegisters(registers Registers) {
	for id, value := range registers {
		s.Registers[id] = value
	}
}

// UpdateTransactionStatus updates the status of a single transaction.
func (s *WorldState) UpdateTransactionStatus(h crypto.Hash, status TxStatus) {
	tx := s.Transactions[h]
	tx.Status = status
	s.Transactions[h] = tx
}

// SealBlock seals a block on the blockchain.
func (s *WorldState) SealBlock(h crypto.Hash) error {
	// TODO: seal the block
	return nil
}
