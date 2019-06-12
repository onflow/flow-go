package data

import (
	"github.com/dapperlabs/bamboo-emulator/crypto"
)

// WorldState represents the current state of the blockchain.
type WorldState struct {
	Blocks       map[crypto.Hash]Block
	Collections  map[crypto.Hash]Collection
	Transactions map[crypto.Hash]Transaction
	Blockchain   []Block
}

// NewWorldState instantiates a new state object with a genesis block.
func NewWorldState() *WorldState {
	blocks := make(map[crypto.Hash]Block)
	collections := make(map[crypto.Hash]Collection)
	txs := make(map[crypto.Hash]Transaction)
	
	genesis := MintGenesisBlock()

	chain := []Block{*genesis}
	blocks[genesis.Hash()] = *genesis

	return &WorldState{
		Blocks: blocks,
		Collections: collections,
		Transactions: txs,
		Blockchain: chain,
	}
}

func (s *WorldState) GetLatestBlock() *Block {
	currHeight := len(s.Blockchain)
	return &s.Blockchain[currHeight-1]
}

func (s *WorldState) GetBlockByNumber(n uint64) (*Block, error) {
	currHeight := len(s.Blockchain)
	if int(n) < currHeight {
		return &s.Blockchain[n], nil
	}

	return nil, &InvalidBlockNumberError{blockNumber: n}
}

func (s *WorldState) GetBlockByHash(h crypto.Hash) (*Block, error) {
	if block, ok := s.Blocks[h]; ok {
		return &block, nil
	}

	return nil, &ItemNotFoundError{hash: h}
}

func (s *WorldState) GetCollection(h crypto.Hash) (*Collection, error) {
	if collection, ok := s.Collections[h]; ok {
		return &collection, nil
	}

	return nil, &ItemNotFoundError{hash: h}
}

func (s *WorldState) GetTransaction(h crypto.Hash) (*Transaction, error) {
	if tx, ok := s.Transactions[h]; ok {
		return &tx, nil
	}

	return nil, &ItemNotFoundError{hash: h}
}

func (s *WorldState) AddBlock(block *Block) error {
	// TODO: add to block map and chain
	return nil
}

func (s *WorldState) InsertCollection(col *Collection) error {
	// TODO: add to collection map
	return nil
}

func (s *WorldState) InsertTransaction(tx *Transaction) error {
	if _, exists := s.Transactions[tx.Hash()]; exists {
		return &DuplicateItemError{hash: tx.Hash()}
	}

	s.Transactions[tx.Hash()] = *tx

	return nil
}

func (s *WorldState) SealBlock(h crypto.Hash) error {
	block, err := s.GetBlockByHash(h)

	if err != nil {
		return err
	}

	(*block).Status = BlockSealed

	return nil
}
