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
	GetLatestBlock() (Block)

	GetBlockByNumber(uint64) (Block, error)
	GetBlockByHash(Hash) (Block, error)
	GetCollection(Hash) (Collection, error)
	GetTransaction(Hash) (Transaction, error)

	AddBlock(Block) (error)
	AddCollection(Collection) (error)
	AddTransaction(Transaction) (error)

	SealBlock(Hash) (error)
}

func (s state) GetLatestBlock() (Block) {
	currHeight := len(s.Blockchain)
	return s.Blockchain[currHeight - 1]
}

func (s state) GetBlockByNumber(n uint64) (Block, error) {
	currHeight := len(s.Blockchain)
	if (int(n) < currHeight) {
		return s.Blockchain[n], nil
	}

	return Block{}, errors.New("invalid Block number: Block number exceeds blockchain length")
}

func (s state) GetBlockByHash(h Hash) (Block, error) {
	if block, ok := s.Blocks[h]; ok {
		return block, nil
	} 

	return Block{}, errors.New("invalid Block hash: Block doesn't exist")

}

func (s state) GetCollection(h Hash) (Collection, error) {
	if collection, ok := s.Collections[h]; ok {
		return collection, nil
	}
		
	return Collection{}, errors.New("invalid Collection hash: Collection doesn't exist")
}

func (s state) GetTransaction(h Hash) (Transaction, error) {
	if tx, ok := s.Transactions[h]; ok {
		return tx, nil
	}

	return Transaction{}, errors.New("invalid Transaction hash: Transaction doesn't exist")
}

func (s state) AddBlock(block Block) (error) {
	// TODO: add to block map and chain
	return nil
}

func (s state) AddCollection(col Collection) (error) {
	// TODO: add to collection map
	return nil
}

func (s state) AddTransaction(tx Transaction) (error) {
	// TODO: add to transaction map
	return nil
}

func (s state) SealBlock(h Hash) (error) {
	// TODO: seal the block
	return nil
}
