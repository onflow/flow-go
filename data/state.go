package data

import (
	"errors"

	"github.com/dapperlabs/bamboo-emulator/crypto"
)

// state represents the current state of the blockchain.
type state struct {
	Blocks       map[crypto.Hash]Block
	Collections  map[crypto.Hash]Collection
	Transactions map[crypto.Hash]Transaction
	Registers    map[string][]byte
	Blockchain   []Block
}

// WorldState exposes an interface to access the state struct.
type WorldState interface {
	GetBlockByNumber(uint64) (*Block, error)
	GetBlockByHash(crypto.Hash) (*Block, error)
	GetLatestBlock() *Block
	GetCollection(crypto.Hash) (*Collection, error)
	GetTransaction(crypto.Hash) (*Transaction, error)
	GetRegister(string) []byte

	AddBlock(*Block) error
	InsertCollection(*Collection) error
	InsertTransaction(*Transaction) error
	CommitRegisters(Registers)

	UpdateTransactionStatus(hash crypto.Hash, status TxStatus)

	SealBlock(crypto.Hash) error
}

// NewWorldState returns a new empty world state.
func NewWorldState() WorldState {
	return &state{
		Blocks:       make(map[crypto.Hash]Block),
		Collections:  make(map[crypto.Hash]Collection),
		Transactions: make(map[crypto.Hash]Transaction),
		Blockchain:   make([]Block, 0),
	}
}

func (s *state) GetLatestBlock() *Block {
	currHeight := len(s.Blockchain)
	return &s.Blockchain[currHeight-1]
}

func (s *state) GetBlockByNumber(n uint64) (*Block, error) {
	currHeight := len(s.Blockchain)
	if int(n) < currHeight {
		return &s.Blockchain[n], nil
	}

	return &Block{}, errors.New("invalid Block number: Block number exceeds blockchain length")
}

func (s *state) GetBlockByHash(h crypto.Hash) (*Block, error) {
	if block, ok := s.Blocks[h]; ok {
		return &block, nil
	}

	return &Block{}, errors.New("invalid Block hash: Block doesn't exist")

}

func (s *state) GetCollection(h crypto.Hash) (*Collection, error) {
	if collection, ok := s.Collections[h]; ok {
		return &collection, nil
	}

	return &Collection{}, errors.New("invalid Collection hash: Collection doesn't exist")
}

func (s *state) GetTransaction(h crypto.Hash) (*Transaction, error) {
	if tx, ok := s.Transactions[h]; ok {
		return &tx, nil
	}

	return &Transaction{}, errors.New("invalid Transaction hash: Transaction doesn't exist")
}

func (s *state) GetRegister(id string) []byte {
	return s.Registers[id]
}

func (s *state) AddBlock(block *Block) error {
	// TODO: add to block map and chain
	return nil
}

func (s *state) InsertCollection(col *Collection) error {
	// TODO: add to collection map
	return nil
}

func (s *state) InsertTransaction(tx *Transaction) error {
	if _, exists := s.Transactions[tx.Hash()]; exists {
		return errors.New("transaction exists")
	}

	s.Transactions[tx.Hash()] = *tx

	return nil
}

func (s *state) CommitRegisters(registers Registers) {
	for id, value := range registers {
		s.Registers[id] = value
	}
}

func (s *state) UpdateTransactionStatus(h crypto.Hash, status TxStatus) {
	tx := s.Transactions[h]
	tx.Status = status
	s.Transactions[h] = tx
}

func (s *state) SealBlock(h crypto.Hash) error {
	// TODO: seal the block
	return nil
}
