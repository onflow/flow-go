package data

import (
	"github.com/sirupsen/logrus"

	"github.com/dapperlabs/bamboo-emulator/crypto"
)

// WorldState represents the current state of the blockchain.
type WorldState struct {
	Blocks       map[crypto.Hash]Block
	Collections  map[crypto.Hash]Collection
	Transactions map[crypto.Hash]Transaction
	Registers    map[string][]byte
	Blockchain   []crypto.Hash
	Log          *logrus.Logger
}

// NewWorldState instantiates a new state object with a genesis block.
func NewWorldState(log *logrus.Logger) *WorldState {
	blocks := make(map[crypto.Hash]Block)
	collections := make(map[crypto.Hash]Collection)
	txs := make(map[crypto.Hash]Transaction)
	registers := make(map[string][]byte)

	genesis := MintGenesisBlock()

	chain := []crypto.Hash{genesis.Hash()}
	blocks[genesis.Hash()] = *genesis

	return &WorldState{
		Blocks:       blocks,
		Collections:  collections,
		Transactions: txs,
		Registers:    registers,
		Blockchain:   chain,
		Log:          log,
	}
}

// GetLatestBlock gets the most recent block in the blockchain.
func (s *WorldState) GetLatestBlock() *Block {
	currHeight := len(s.Blockchain)
	blockHash := s.Blockchain[currHeight-1]
	block, _ := s.GetBlockByHash(blockHash)
	return block
}

// GetBlockByNumber gets a block by number.
func (s *WorldState) GetBlockByNumber(n uint64) (*Block, error) {
	currHeight := len(s.Blockchain)
	if int(n) < currHeight {
		blockHash := s.Blockchain[n]
		return s.GetBlockByHash(blockHash)
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
	if _, exists := s.Blocks[block.Hash()]; exists {
		return &DuplicateItemError{hash: block.Hash()}
	}

	s.Blocks[block.Hash()] = *block
	s.Blockchain = append(s.Blockchain, block.Hash())

	return nil
}

// InsertCollection inserts a new collection into the state.
func (s *WorldState) InsertCollection(col *Collection) error {
	if _, exists := s.Collections[col.Hash()]; exists {
		return &DuplicateItemError{hash: col.Hash()}
	}

	s.Collections[col.Hash()] = *col

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
func (s *WorldState) UpdateTransactionStatus(h crypto.Hash, status TxStatus) error {
	tx, err := s.GetTransaction(h)
	if err != nil {
		return err
	}

	tx.Status = status

	s.Transactions[h] = *tx

	return nil
}

// SealBlock seals a block on the blockchain.
func (s *WorldState) SealBlock(h crypto.Hash) error {
	block, err := s.GetBlockByHash(h)

	if err != nil {
		return err
	}

	block.Status = BlockSealed

	return nil
}
