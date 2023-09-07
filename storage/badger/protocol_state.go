package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// ProtocolState implements persistent storage for storing identity table instances.
// Protocol state uses an embedded cache without storing capabilities(store happens on first retrieval) to avoid unnecessary
// operations and to speed up access to frequently used identity tables.
// TODO: update naming to IdentityTable
type ProtocolState struct {
	db    *badger.DB
	cache *Cache
}

var _ storage.ProtocolState = (*ProtocolState)(nil)

// NewProtocolState creates a ProtocolState instance, which is a database of identity table instances.
// It supports storing, caching and retrieving by ID or the additionally indexed block ID.
func NewProtocolState(collector module.CacheMetrics,
	epochSetups storage.EpochSetups,
	epochCommits storage.EpochCommits,
	db *badger.DB,
	cacheSize uint,
) *ProtocolState {
	retrieve := func(key interface{}) func(tx *badger.Txn) (interface{}, error) {
		protocolStateID := key.(flow.Identifier)
		var protocolStateEntry flow.ProtocolStateEntry
		return func(tx *badger.Txn) (interface{}, error) {
			err := operation.RetrieveProtocolState(protocolStateID, &protocolStateEntry)(tx)
			if err != nil {
				return nil, err
			}
			result, err := newRichProtocolStateEntry(&protocolStateEntry, epochSetups, epochCommits)
			if err != nil {
				return nil, fmt.Errorf("could not create rich identity table entry: %w", err)
			}
			return result, nil
		}
	}

	return &ProtocolState{
		db: db,
		cache: newCache(collector, metrics.ResourceProtocolState,
			withLimit(cacheSize),
			withStore(noopStore),
			withRetrieve(retrieve)),
	}
}

// StoreTx allows us to store an identity table as part of a DB tx, while still going through the caching layer.
// Per convention, the given Identity Table must be in canonical order, otherwise an exception is returned.
// Expected error returns during normal operations:
//   - storage.ErrAlreadyExists if an Identity Table with the given id is already stored
func (s *ProtocolState) StoreTx(id flow.Identifier, protocolState *flow.ProtocolStateEntry) func(*transaction.Tx) error {
	return func(tx *transaction.Tx) error {
		if !protocolState.Identities.Sorted(order.IdentifierCanonical) {
			return fmt.Errorf("sanity check failed: identities are not sorted")
		}
		if protocolState.NextEpochProtocolState != nil {
			if !protocolState.NextEpochProtocolState.Identities.Sorted(order.IdentifierCanonical) {
				return fmt.Errorf("sanity check failed: next epoch identities are not sorted")
			}
		}
		return transaction.WithTx(operation.InsertProtocolState(id, protocolState))(tx)
	}
}

// Index indexes the identity table by block ID.
// Error returns:
//   - storage.ErrAlreadyExists if an identity table for the given blockID has already been indexed
func (s *ProtocolState) Index(blockID flow.Identifier, protocolStateID flow.Identifier) func(*transaction.Tx) error {
	return func(tx *transaction.Tx) error {
		err := transaction.WithTx(operation.IndexProtocolState(blockID, protocolStateID))(tx)
		if err != nil {
			return fmt.Errorf("could not index identity table for block (%x): %w", blockID[:], err)
		}
		return nil
	}
}

// ByID retrieves the identity table by its ID.
// Error returns:
//   - storage.ErrNotFound if no identity table with the given ID exists
func (s *ProtocolState) ByID(id flow.Identifier) (*flow.RichProtocolStateEntry, error) {
	tx := s.db.NewTransaction(false)
	defer tx.Discard()
	return s.byID(id)(tx)
}

// ByBlockID retrieves the identity table by the respective block ID.
// TODO: clarify whether the blockID is the block that defines this identity table or the _child_ block where the identity table is applied. CAUTION: surface for bugs!
// Error returns:
//   - storage.ErrNotFound if no identity table for the given blockID exists
func (s *ProtocolState) ByBlockID(blockID flow.Identifier) (*flow.RichProtocolStateEntry, error) {
	tx := s.db.NewTransaction(false)
	defer tx.Discard()
	return s.byBlockID(blockID)(tx)
}

// byID retrieves the identity table by its ID. Error returns:
//   - storage.ErrNotFound if no identity table with the given ID exists
func (s *ProtocolState) byID(protocolStateID flow.Identifier) func(*badger.Txn) (*flow.RichProtocolStateEntry, error) {
	return func(tx *badger.Txn) (*flow.RichProtocolStateEntry, error) {
		val, err := s.cache.Get(protocolStateID)(tx)
		if err != nil {
			return nil, err
		}
		return val.(*flow.RichProtocolStateEntry), nil
	}
}

// byBlockID retrieves the identity table by the respective block ID.
// TODO: clarify whether the blockID is the block that defines this identity table or the _child_ block where the identity table is applied. CAUTION: surface for bugs!
// Error returns:
//   - storage.ErrNotFound if no identity table for the given blockID exists
func (s *ProtocolState) byBlockID(blockID flow.Identifier) func(*badger.Txn) (*flow.RichProtocolStateEntry, error) {
	return func(tx *badger.Txn) (*flow.RichProtocolStateEntry, error) {
		var protocolStateID flow.Identifier
		err := operation.LookupProtocolState(blockID, &protocolStateID)(tx)
		if err != nil {
			return nil, fmt.Errorf("could not lookup identity table ID for block (%x): %w", blockID[:], err)
		}
		return s.byID(protocolStateID)(tx)
	}
}

// newRichProtocolStateEntry constructs a rich protocol state entry from a protocol state entry.
// It queries and fills in epoch setups and commits for previous and current epochs and possibly next epoch.
// No errors are expected during normal operation.
func newRichProtocolStateEntry(
	protocolState *flow.ProtocolStateEntry,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
) (*flow.RichProtocolStateEntry, error) {
	var (
		previousEpochSetup  *flow.EpochSetup
		previousEpochCommit *flow.EpochCommit
		nextEpochSetup      *flow.EpochSetup
		nextEpochCommit     *flow.EpochCommit
		err                 error
	)
	// query and fill in epoch setups and commits for previous and current epochs
	if protocolState.PreviousEpochEventIDs.SetupID != flow.ZeroID {
		previousEpochSetup, err = setups.ByID(protocolState.PreviousEpochEventIDs.SetupID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve previous epoch setup: %w", err)
		}
		previousEpochCommit, err = commits.ByID(protocolState.PreviousEpochEventIDs.CommitID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve previous epoch commit: %w", err)
		}
	}

	currentEpochSetup, err := setups.ByID(protocolState.CurrentEpochEventIDs.SetupID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve current epoch setup: %w", err)
	}
	currentEpochCommit, err := commits.ByID(protocolState.CurrentEpochEventIDs.CommitID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve current epoch commit: %w", err)
	}

	// if next epoch has been already committed, fill in data for it as well.
	if protocolState.NextEpochProtocolState != nil {
		nextEpochProtocolState := *protocolState.NextEpochProtocolState
		nextEpochSetup, err = setups.ByID(nextEpochProtocolState.CurrentEpochEventIDs.SetupID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve next epoch setup: %w", err)
		}
		if nextEpochProtocolState.CurrentEpochEventIDs.CommitID != flow.ZeroID {
			nextEpochCommit, err = commits.ByID(nextEpochProtocolState.CurrentEpochEventIDs.CommitID)
			if err != nil {
				return nil, fmt.Errorf("could not retrieve next epoch commit: %w", err)
			}
		}
	}

	return flow.NewRichProtocolStateEntry(
		protocolState,
		previousEpochSetup,
		previousEpochCommit,
		currentEpochSetup,
		currentEpochCommit,
		nextEpochSetup,
		nextEpochCommit,
	)
}
