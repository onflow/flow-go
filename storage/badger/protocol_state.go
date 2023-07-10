package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// ProtocolState implements persistent storage for storing entities of protocol state.
type ProtocolState struct {
	db    *badger.DB
	cache *Cache
}

var _ storage.ProtocolState = (*ProtocolState)(nil)

// NewProtocolState Creates ProtocolState instance which is a database of protocol state entries
// which supports storing, caching and retrieving by ID and additionally indexed block ID.
func NewProtocolState(collector module.CacheMetrics, db *badger.DB, cacheSize uint) *ProtocolState {
	store := func(key interface{}, val interface{}) func(*transaction.Tx) error {
		id := key.(flow.Identifier)
		protocolStateEntry := val.(*flow.ProtocolStateEntry)
		return transaction.WithTx(operation.InsertProtocolState(id, protocolStateEntry))
	}

	retrieve := func(key interface{}) func(tx *badger.Txn) (interface{}, error) {
		protocolStateID := key.(flow.Identifier)
		var protocolStateEntry flow.ProtocolStateEntry
		return func(tx *badger.Txn) (interface{}, error) {
			err := operation.RetrieveProtocolState(protocolStateID, &protocolStateEntry)(tx)
			return &protocolStateEntry, err
		}
	}

	return &ProtocolState{
		db: db,
		cache: newCache(collector, metrics.ResourceProtocolState,
			withLimit(cacheSize),
			withStore(store),
			withRetrieve(retrieve)),
	}
}

// StoreTx allows us to store protocol state as part of a DB tx, while still going through the caching layer.
func (s *ProtocolState) StoreTx(id flow.Identifier, protocolState *flow.ProtocolStateEntry) func(*transaction.Tx) error {
	return s.cache.PutTx(id, protocolState)
}

// Index indexes the protocol state by block ID.
func (s *ProtocolState) Index(blockID flow.Identifier, protocolStateID flow.Identifier) func(*transaction.Tx) error {
	return func(tx *transaction.Tx) error {
		err := transaction.WithTx(operation.IndexProtocolState(blockID, protocolStateID))(tx)
		if err != nil {
			return fmt.Errorf("could not index protocol state for block (%x): %w", blockID[:], err)
		}
		return nil
	}
}

// ByID returns the protocol state by its ID.
func (s *ProtocolState) ByID(id flow.Identifier) (*flow.ProtocolStateEntry, error) {
	tx := s.db.NewTransaction(false)
	defer tx.Discard()
	return s.byID(id)(tx)
}

// ByBlockID returns the protocol state by block ID.
func (s *ProtocolState) ByBlockID(blockID flow.Identifier) (*flow.ProtocolStateEntry, error) {
	tx := s.db.NewTransaction(false)
	defer tx.Discard()
	return s.byBlockID(blockID)(tx)
}

func (s *ProtocolState) byID(protocolStateID flow.Identifier) func(*badger.Txn) (*flow.ProtocolStateEntry, error) {
	return func(tx *badger.Txn) (*flow.ProtocolStateEntry, error) {
		val, err := s.cache.Get(protocolStateID)(tx)
		if err != nil {
			return nil, err
		}
		return val.(*flow.ProtocolStateEntry), nil
	}
}

func (s *ProtocolState) byBlockID(blockID flow.Identifier) func(*badger.Txn) (*flow.ProtocolStateEntry, error) {
	return func(tx *badger.Txn) (*flow.ProtocolStateEntry, error) {
		var protocolStateID flow.Identifier
		err := operation.LookupProtocolState(blockID, &protocolStateID)(tx)
		if err != nil {
			return nil, fmt.Errorf("could not lookup protocol state ID for block (%x): %w", blockID[:], err)
		}
		return s.byID(protocolStateID)(tx)
	}
}
