package data

import (
	"errors"
)

// state represents the current state of the blockchain.
type state struct {
	Blocks       map[Hash]Block
	Collections  map[Hash]Collection
	Transactions map[Hash]Transaction
	Blockchain   []Block
}

// WorldState exposes an interface to access the state struct.
type WorldState interface {
	GetBlockByNumber(uint64) (*Block, error)
	GetBlockByHash(Hash) (*Block, error)
	GetLatestBlock() *Block
	GetCollection(Hash) (*Collection, error)
	GetTransaction(Hash) (*Transaction, error)

	AddBlock(*Block) error
	InsertCollection(*Collection) error
	InsertTransaction(*Transaction) error

	SealBlock(Hash) error
}

// NewWorldState returns a new empty world state.
func NewWorldState() WorldState {
	return &state{
		Blocks:       make(map[Hash]Block),
		Collections:  make(map[Hash]Collection),
		Transactions: make(map[Hash]Transaction),
		Blockchain:   make([]Block, 0),
	}
}

func (s state) GetLatestBlock() *Block {
	currHeight := len(s.Blockchain)
	return &s.Blockchain[currHeight-1]
}

func (s state) GetBlockByNumber(n uint64) (*Block, error) {
	currHeight := len(s.Blockchain)
	if int(n) < currHeight {
		return &s.Blockchain[n], nil
	}

	return &Block{}, errors.New("invalid Block number: Block number exceeds blockchain length")
}

func (s state) GetBlockByHash(h Hash) (*Block, error) {
	if block, ok := s.Blocks[h]; ok {
		return &block, nil
	}

	return &Block{}, errors.New("invalid Block hash: Block doesn't exist")

}

func (s state) GetCollection(h Hash) (*Collection, error) {
	if collection, ok := s.Collections[h]; ok {
		return &collection, nil
	}

	return &Collection{}, errors.New("invalid Collection hash: Collection doesn't exist")
}

func (s state) GetTransaction(h Hash) (*Transaction, error) {
	if tx, ok := s.Transactions[h]; ok {
		return &tx, nil
	}

	return &Transaction{}, errors.New("invalid Transaction hash: Transaction doesn't exist")
}

func (s state) AddBlock(block *Block) error {
	// TODO: add to block map and chain
	return nil
}

func (s state) InsertCollection(col *Collection) error {
	// TODO: add to collection map
	return nil
}

func (s state) InsertTransaction(tx *Transaction) error {
	if _, exists := s.Transactions[tx.Hash()]; exists {
		return errors.New("transaction exists")
	}

	s.Transactions[tx.Hash()] = *tx

	return nil
}

func (s state) SealBlock(h Hash) error {
	// TODO: seal the block
	return nil
}
