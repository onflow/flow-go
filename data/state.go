package data

import (
	"github.com/dapperlabs/bamboo-emulator/crypto"
)

// WorldState represents the current state of the blockchain.
type WorldState struct {
	Blocks       map[crypto.Hash]Block
	Collections  map[crypto.Hash]Collection
	Transactions map[crypto.Hash]Transaction
	Blockchain   []crypto.Hash
}

// NewWorldState instantiates a new state object with a genesis block.
func NewWorldState() *WorldState {
	blocks := make(map[crypto.Hash]Block)
	collections := make(map[crypto.Hash]Collection)
	txs := make(map[crypto.Hash]Transaction)

	genesis := MintGenesisBlock()

	chain := []crypto.Hash{genesis.Hash()}
	blocks[genesis.Hash()] = *genesis

	return &WorldState{
		Blocks:       blocks,
		Collections:  collections,
		Transactions: txs,
		Blockchain:   chain,
	}
}

func (s *WorldState) GetLatestBlock() (*Block, error) {
	currHeight := len(s.Blockchain)
	blockHash := s.Blockchain[currHeight-1]
	return s.GetBlockByHash(blockHash)
}

func (s *WorldState) GetBlockByNumber(n uint64) (*Block, error) {
	currHeight := len(s.Blockchain)
	if int(n) < currHeight {
		blockHash := s.Blockchain[n]
		return s.GetBlockByHash(blockHash)
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
	if _, exists := s.Blocks[block.Hash()]; exists {
		return &DuplicateItemError{hash: block.Hash()}
	}

	s.Blocks[block.Hash()] = *block
	s.Blockchain = append(s.Blockchain, block.Hash())

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
