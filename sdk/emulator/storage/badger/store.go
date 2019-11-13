package badger

import (
	"errors"
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

// Store is an embedded storage implementation using Badger as the underlying
// persistent key-value store.
type Store struct {
	db *badger.DB
}

// New returns a new Badger Store.
func New(config *Config) (storage.Store, error) {
	db, err := badger.Open(badger.DefaultOptions(config.Path))
	if err != nil {
		return nil, fmt.Errorf("could not open database: %w", err)
	}
	return Store{db}, nil
}

func (s Store) GetBlockByHash(blockHash crypto.Hash) (block types.Block, err error) {
	err = s.db.View(func(txn *badger.Txn) error {
		// get block number by block hash
		encBlockNumber, err := getTx(txn)(blockHashIndexKey(blockHash))
		if err != nil {
			return err
		}

		// decode block number
		var blockNumber uint64
		if err := decodeUint64(&blockNumber, encBlockNumber); err != nil {
			return err
		}

		// get block by block number and decode
		encBlock, err := getTx(txn)(blockKey(blockNumber))
		if err != nil {
			return err
		}
		return decodeBlock(&block, encBlock)
	})
	return
}

func (s Store) GetBlockByNumber(blockNumber uint64) (block types.Block, err error) {
	err = s.db.View(func(txn *badger.Txn) error {
		encBlock, err := getTx(txn)(blockKey(blockNumber))
		if err != nil {
			return err
		}
		return decodeBlock(&block, encBlock)
	})
	return
}

func (s Store) GetLatestBlock() (block types.Block, err error) {
	err = s.db.View(func(txn *badger.Txn) error {
		// get latest block number
		latestBlockNumber, err := getLatestBlockNumberTx(txn)
		if err != nil {
			return err
		}

		// get corresponding block
		encBlock, err := getTx(txn)(blockKey(latestBlockNumber))
		if err != nil {
			return err
		}
		return decodeBlock(&block, encBlock)
	})
	return
}

func (s Store) InsertBlock(block types.Block) error {
	encBlock, err := encodeBlock(block)
	if err != nil {
		return err
	}
	encBlockNumber, err := encodeUint64(block.Number)
	if err != nil {
		return err
	}

	return s.db.Update(func(txn *badger.Txn) error {
		// get latest block number
		latestBlockNumber, err := getLatestBlockNumberTx(txn)
		if err != nil {
			return err
		}

		// insert the block by block number
		if err := txn.Set(blockKey(block.Number), encBlock); err != nil {
			return err
		}
		// add block hash to hash->number lookup
		if err := txn.Set(blockHashIndexKey(block.Hash()), encBlockNumber); err != nil {
			return err
		}

		// if this is latest block, set latest block
		if block.Number > latestBlockNumber {
			return txn.Set(latestBlockKey(), encBlockNumber)
		}
		return nil
	})
}

func (s Store) GetTransaction(txHash crypto.Hash) (tx flow.Transaction, err error) {
	err = s.db.View(func(txn *badger.Txn) error {
		encTx, err := getTx(txn)(transactionKey(txHash))
		if err != nil {
			return err
		}
		return decodeTransaction(&tx, encTx)
	})
	return
}

func (s Store) InsertTransaction(tx flow.Transaction) error {
	encTx, err := encodeTransaction(tx)
	if err != nil {
		return err
	}

	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(transactionKey(tx.Hash()), encTx)
	})
}

func (s Store) GetRegistersView(blockNumber uint64) (view flow.RegistersView, err error) {
	err = s.db.View(func(txn *badger.Txn) error {
		encRegisters, err := getTx(txn)(registersKey(blockNumber))
		if err != nil {
			return err
		}

		var registers flow.Registers
		if err := decodeRegisters(&registers, encRegisters); err != nil {
			return err
		}
		view = *registers.NewView()
		return nil
	})
	return
}

func (s Store) SetRegisters(blockNumber uint64, registers flow.Registers) error {
	encRegisters, err := encodeRegisters(registers)
	if err != nil {
		return err
	}

	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(registersKey(blockNumber), encRegisters)
	})
}

func (s Store) GetEvents(eventType string, startBlock, endBlock uint64) (events []flow.Event, err error) {
	// set up an iterator over all events
	iterOpts := badger.DefaultIteratorOptions
	iterOpts.Prefix = []byte(eventsKeyPrefix)

	err = s.db.View(func(txn *badger.Txn) error {
		iter := txn.NewIterator(iterOpts)
		defer iter.Close()
		// seek the iterator to the start block
		iter.Seek(eventsKey(startBlock))
		// create a buffer for copying events, this is reused for each block
		eventBuf := make([]byte, 256)

		for iter.Rewind(); iter.Valid(); iter.Next() {
			item := iter.Item()
			// ensure the events are within the block number range
			blockNumber := blockNumberFromEventsKey(item.Key())
			if blockNumber < startBlock || blockNumber > endBlock {
				break
			}

			// decode the events from this block
			encEvents, err := item.ValueCopy(eventBuf)
			if err != nil {
				return err
			}
			var blockEvents flow.EventList
			if err := decodeEventList(&blockEvents, encEvents); err != nil {
				return err
			}

			if eventType == "" {
				// if no type filter specified, add all block events
				events = append(events, blockEvents...)
			} else {
				// otherwise filter by event type
				for _, event := range blockEvents {
					if event.Type == eventType {
						events = append(events, event)
					}
				}
			}
		}
		return nil
	})
	return
}

func (s Store) InsertEvents(blockNumber uint64, events ...flow.Event) error {
	encEvents, err := encodeEventList(events)
	if err != nil {
		return err
	}

	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(eventsKey(blockNumber), encEvents)
	})
}

// getTx returns a getter function bound to the input transaction that can be
// used to get values from Badger.
//
// The getter function checks for key-not-found errors and wraps them in
// storage.NotFound in order to comply with the storage.Store interface.
//
// This saves a few lines of converting a badger.Item to []byte.
func getTx(txn *badger.Txn) func([]byte) ([]byte, error) {
	return func(key []byte) ([]byte, error) {
		// Badger returns an "item" upon GETs, we need to copy the actual value
		// from the item and return it.
		item, err := txn.Get(key)
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return nil, storage.ErrNotFound{}
			}
			return nil, err
		}

		val := make([]byte, item.ValueSize())
		return item.ValueCopy(val)
	}
}

// getLatestBlockNumberTx retrieves the latest block number and returns it.
// Must be called from within a Badger transaction.
func getLatestBlockNumberTx(txn *badger.Txn) (uint64, error) {
	encBlockNumber, err := getTx(txn)(latestBlockKey())
	if err != nil {
		return 0, err
	}

	var blockNumber uint64
	if err := decodeUint64(&blockNumber, encBlockNumber); err != nil {
		return 0, err
	}

	return blockNumber, nil
}
