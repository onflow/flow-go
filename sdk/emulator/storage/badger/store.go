package badger

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/emulator/storage"
	"github.com/dapperlabs/flow-go/sdk/emulator/types"

	"github.com/dgraph-io/badger"
)

type Config struct {
	Path string
}

type Option func(*Config)

// WithPath sets the path of the Badger database.
func WithPath(path string) Option {
	return func(config *Config) {
		config.Path = path
	}
}

// BadgerStore is an embedded storage implementation.
type Store struct {
	db *badger.DB
}

func New(config *Config) (storage.Store, error) {
	db, err := badger.Open(badger.DefaultOptions(config.Path))
	if err != nil {
		return nil, fmt.Errorf("could not open database: %w", err)
	}
	return Store{db}, nil
}

func (s Store) GetBlockByHash(crypto.Hash) (types.Block, error) {
	panic("implement me")
}

func (s Store) GetBlockByNumber(blockNumber uint64) (types.Block, error) {
	panic("implement me")
}

func (s Store) GetLatestBlock() (types.Block, error) {
	panic("implement me")
}

func (s Store) InsertBlock(block types.Block) error {
	s.db.Update(func(txn *badger.Txn) error {
		txn.Set(blockKey(block.Number), []byte{})
	})
}

func (s Store) GetTransaction(crypto.Hash) (flow.Transaction, error) {
	panic("implement me")
}

func (s Store) InsertTransaction(flow.Transaction) error {
	panic("implement me")
}

func (s Store) GetRegistersView(blockNumber uint64) (flow.RegistersView, error) {
	panic("implement me")
}

func (s Store) SetRegisters(blockNumber uint64, registers flow.Registers) error {
	panic("implement me")
}

func (s Store) GetEvents(blockNumber uint64, eventType string, startBlock, endBlock uint64) ([]flow.Event, error) {
	panic("implement me")
}

func (s Store) InsertEvents(blockNumber uint64, events ...flow.Event) error {
	panic("implement me")
}

func blockKey(blockNumber uint64) []byte {
	return []byte(fmt.Sprintf("block_by_number-%d", blockNumber))
}

func blockHashIndexKey(blockHash crypto.Hash) []byte {
	return []byte(fmt.Sprintf("block_hash_to_number-%s", blockHash.Hex()))
}

func transactionKey(txHash crypto.Hash) []byte {
	return []byte(fmt.Sprintf("transaction_by_hash-%s", txHash.Hex()))
}

func registersKey(blockNumber uint64) []byte {
	return []byte(fmt.Sprintf("registers_by_block_number-%d", blockNumber))
}

func eventsKey(blockNumber uint64) []byte {
	return []byte(fmt.Sprintf("events_by_block_number-%d", blockNumber))
}
