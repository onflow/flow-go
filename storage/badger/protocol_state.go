package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
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
	db             *badger.DB
	cache          *Cache[flow.Identifier, *flow.RichProtocolStateEntry] // protocol_state_id -> protocol state
	byBlockIdCache *Cache[flow.Identifier, flow.Identifier]              // block id -> protocol_state_id
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
	retrieve := func(protocolStateID flow.Identifier) func(tx *badger.Txn) (*flow.RichProtocolStateEntry, error) {
		var protocolStateEntry flow.ProtocolStateEntry
		return func(tx *badger.Txn) (*flow.RichProtocolStateEntry, error) {
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

	retrieveByBlockID := func(blockID flow.Identifier) func(tx *badger.Txn) (flow.Identifier, error) {
		return func(tx *badger.Txn) (flow.Identifier, error) {
			var protocolStateID flow.Identifier
			err := operation.LookupProtocolState(blockID, &protocolStateID)(tx)
			if err != nil {
				return flow.ZeroID, fmt.Errorf("could not lookup identity table ID for block (%x): %w", blockID[:], err)
			}
			return protocolStateID, nil
		}
	}

	return &ProtocolState{
		db: db,
		cache: newCache[flow.Identifier, *flow.RichProtocolStateEntry](collector, metrics.ResourceProtocolState,
			withLimit[flow.Identifier, *flow.RichProtocolStateEntry](cacheSize),
			withStore(noopStore[flow.Identifier, *flow.RichProtocolStateEntry]),
			withRetrieve(retrieve)),
		byBlockIdCache: newCache[flow.Identifier, flow.Identifier](collector, metrics.ResourceProtocolStateByBlockID,
			withLimit[flow.Identifier, flow.Identifier](cacheSize),
			withStore(noopStore[flow.Identifier, flow.Identifier]),
			withRetrieve(retrieveByBlockID)),
	}
}

// StoreTx allows us to store an identity table as part of a DB tx, while still going through the caching layer.
// Per convention, the given Identity Table must be in canonical order, otherwise an exception is returned.
// Expected error returns during normal operations:
//   - storage.ErrAlreadyExists if an Identity Table with the given id is already stored
func (s *ProtocolState) StoreTx(id flow.Identifier, protocolState *flow.ProtocolStateEntry) func(*transaction.Tx) error {
	// front-load sanity checks:
	if !protocolState.CurrentEpoch.ActiveIdentities.Sorted(order.IdentifierCanonical) {
		return transaction.Fail(fmt.Errorf("sanity check failed: identities are not sorted"))
	}
	if protocolState.NextEpoch != nil && !protocolState.NextEpoch.ActiveIdentities.Sorted(order.IdentifierCanonical) {
		return transaction.Fail(fmt.Errorf("sanity check failed: next epoch identities are not sorted"))
	}

	// happy path: return anonymous function, whose future execution (as part of a transaction) will store the protocolState
	return transaction.WithTx(operation.InsertProtocolState(id, protocolState))
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

// byID retrieves the identity table by its ID. Error returns:
//   - storage.ErrNotFound if no identity table with the given ID exists
func (s *ProtocolState) byID(protocolStateID flow.Identifier) func(*badger.Txn) (*flow.RichProtocolStateEntry, error) {
	return s.cache.Get(protocolStateID)
}

// ByBlockID retrieves the identity table by the respective block ID.
// TODO: clarify whether the blockID is the block that defines this identity table or the _child_ block where the identity table is applied. CAUTION: surface for bugs!
// Error returns:
//   - storage.ErrNotFound if no identity table for the given blockID exists
func (s *ProtocolState) ByBlockID(blockID flow.Identifier) (*flow.RichProtocolStateEntry, error) {
	tx := s.db.NewTransaction(false)
	defer tx.Discard()
	protocolStateID, err := s.byBlockIdCache.Get(blockID)(tx)
	if err != nil {
		return nil, fmt.Errorf("could not lookup identity table ID for block (%x): %w", blockID[:], err)
	}
	return s.byID(protocolStateID)(tx)
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
	if protocolState.PreviousEpoch != nil {
		previousEpochSetup, err = setups.ByID(protocolState.PreviousEpoch.SetupID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve previous epoch setup: %w", err)
		}
		previousEpochCommit, err = commits.ByID(protocolState.PreviousEpoch.CommitID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve previous epoch commit: %w", err)
		}
	}

	currentEpochSetup, err := setups.ByID(protocolState.CurrentEpoch.SetupID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve current epoch setup: %w", err)
	}
	currentEpochCommit, err := commits.ByID(protocolState.CurrentEpoch.CommitID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve current epoch commit: %w", err)
	}

	// if next epoch has been set up, fill in data for it as well
	nextEpoch := protocolState.NextEpoch
	if nextEpoch != nil {
		nextEpochSetup, err = setups.ByID(nextEpoch.SetupID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve next epoch's setup event: %w", err)
		}
		if nextEpoch.CommitID != flow.ZeroID {
			nextEpochCommit, err = commits.ByID(nextEpoch.CommitID)
			if err != nil {
				return nil, fmt.Errorf("could not retrieve next epoch's commit event: %w", err)
			}
		}
	}

	result, err := flow.NewRichProtocolStateEntry(
		protocolState,
		previousEpochSetup,
		previousEpochCommit,
		currentEpochSetup,
		currentEpochCommit,
		nextEpochSetup,
		nextEpochCommit,
	)
	if err != nil {
		// observing an error here would be an indication of severe data corruption or bug in our code since
		// all data should be available and correctly structured at this point.
		return nil, irrecoverable.NewExceptionf("critical failure while instantiating RichProtocolStateEntry: %w", err)
	}
	return result, nil
}
