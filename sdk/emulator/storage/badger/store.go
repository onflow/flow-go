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

// Store is an embedded storage implementation using Badger as the underlying
// persistent key-value store.
type Store struct {
	db              *badger.DB
	ledgerChangeLog changelog
}

// New returns a new Badger Store.
func New(opts ...Opt) (Store, error) {
	badgerOptions := getBadgerOptions(opts...)

	db, err := badger.Open(badgerOptions)
	if err != nil {
		return Store{}, fmt.Errorf("could not open database: %w", err)
	}

	store := Store{db, newChangelog()}
	if err = store.setup(); err != nil {
		return Store{}, err
	}

	return store, nil
}

// setup sets up in-memory indexes and prepares the store for use.
func (s Store) setup() error {
	s.db.RLock()
	defer s.db.RUnlock()

	iterOpts := badger.DefaultIteratorOptions
	// only search for changelog entries
	iterOpts.Prefix = []byte(ledgerChangelogKeyPrefix)
	// create a buffer for copying changelists, this is reused for each register
	clistBuf := make([]byte, 256)

	// read the changelist from disk for each register
	return s.db.View(func(txn *badger.Txn) error {
		iter := txn.NewIterator(iterOpts)
		defer iter.Close()

		for iter.Rewind(); iter.Valid(); iter.Next() {
			item := iter.Item()
			registerID := registerIDFromLedgerChangelogKey(item.Key())
			// ensure the register ID is value
			if len(registerID) == 0 {
				return errors.New("found changelist for invalid register ID")
			}

			// decode the changelist
			encClist, err := item.ValueCopy(clistBuf)
			if err != nil {
				return err
			}
			var clist changelist
			if err := decodeChangelist(&clist, encClist); err != nil {
				return err
			}

			// add to the changelog
			s.ledgerChangeLog.setChangelist(registerID, clist)
		}
		return nil
	})
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
		if err != nil && !errors.Is(err, storage.ErrNotFound{}) {
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
		if block.Number >= latestBlockNumber {
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

func (s Store) GetLedger(blockNumber uint64) (flow.Ledger, error) {
	s.ledgerChangeLog.RLock()
	defer s.ledgerChangeLog.RUnlock()

	ledger := make(flow.Ledger)

	err := s.db.View(func(txn *badger.Txn) error {
		for registerID, clist := range s.ledgerChangeLog.changelists() {
			// Get the block at which the register last changed value. If no
			// such block exists, skip this register.
			lastChangedBlock := clist.search(blockNumber)
			if lastChangedBlock == notFound {
				continue
			}

			// Get the value of the register and add it to the ledger.
			value, err := getTx(txn)(ledgerValueKey(registerID, lastChangedBlock))
			if err != nil {
				return err
			}
			ledger[registerID] = value
		}
		return nil
	})
	return ledger, err
}

func (s Store) SetLedger(blockNumber uint64, ledger flow.Ledger) error {
	s.ledgerChangeLog.Lock()
	defer s.ledgerChangeLog.Unlock()

	return s.db.Update(func(txn *badger.Txn) error {
		for registerID, value := range ledger {
			if err := txn.Set(ledgerValueKey(registerID, blockNumber), value); err != nil {
				return err
			}

			// update the in-memory changelog
			s.ledgerChangeLog.addChange(registerID, blockNumber)

			// encode and write the changelist for the register to disk
			encChangelist, err := encodeChangelist(s.ledgerChangeLog.getChangelist(registerID))
			if err != nil {
				return err
			}
			if err := txn.Set(ledgerChangelogKey(registerID), encChangelist); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s Store) GetEvents(eventType string, startBlock, endBlock uint64) (events []flow.Event, err error) {
	// set up an iterator over all events
	iterOpts := badger.DefaultIteratorOptions
	iterOpts.Prefix = []byte(eventsKeyPrefix)

	err = s.db.View(func(txn *badger.Txn) error {
		iter := txn.NewIterator(iterOpts)
		defer iter.Close()
		// create a buffer for copying events, this is reused for each block
		eventBuf := make([]byte, 256)

		// seek the iterator to the start block before the loop
		iter.Seek(eventsKey(startBlock))
		for ; iter.Valid(); iter.Next() {
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
			var blockEvents []flow.Event
			if err := decodeEvents(&blockEvents, encEvents); err != nil {
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
	encEvents, err := encodeEvents(events)
	if err != nil {
		return err
	}

	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(eventsKey(blockNumber), encEvents)
	})
}

// Close closes the underlying Badger database. It is necessary to close
// a Store before exiting to ensure all writes are persisted to disk.
func (s Store) Close() error {
	return s.db.Close()
}

// Sync syncs database content to disk.
func (s Store) Sync() error {
	return s.db.Sync()
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
