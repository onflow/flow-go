package badger

import (
	"fmt"
	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/flow-go/model/flow/order"

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
func NewProtocolState(collector module.CacheMetrics,
	epochSetups storage.EpochSetups,
	epochCommits storage.EpochCommits,
	db *badger.DB,
	cacheSize uint) *ProtocolState {
	retrieve := func(key interface{}) func(tx *badger.Txn) (interface{}, error) {
		protocolStateID := key.(flow.Identifier)
		var protocolStateEntry flow.ProtocolStateEntry
		return func(tx *badger.Txn) (interface{}, error) {
			err := operation.RetrieveProtocolState(protocolStateID, &protocolStateEntry)(tx)
			if err != nil {
				return nil, err
			}
			result, err := newRichProtocolStateEntry(protocolStateEntry, epochSetups, epochCommits)
			if err != nil {
				return nil, fmt.Errorf("could not create rich protocol state entry: %w", err)
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

// StoreTx allows us to store protocol state as part of a DB tx, while still going through the caching layer.
func (s *ProtocolState) StoreTx(id flow.Identifier, protocolState *flow.ProtocolStateEntry) func(*transaction.Tx) error {
	return transaction.WithTx(operation.InsertProtocolState(id, protocolState))
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
func (s *ProtocolState) ByID(id flow.Identifier) (*flow.RichProtocolStateEntry, error) {
	tx := s.db.NewTransaction(false)
	defer tx.Discard()
	return s.byID(id)(tx)
}

// ByBlockID returns the protocol state by block ID.
func (s *ProtocolState) ByBlockID(blockID flow.Identifier) (*flow.RichProtocolStateEntry, error) {
	tx := s.db.NewTransaction(false)
	defer tx.Discard()
	return s.byBlockID(blockID)(tx)
}

func (s *ProtocolState) byID(protocolStateID flow.Identifier) func(*badger.Txn) (*flow.RichProtocolStateEntry, error) {
	return func(tx *badger.Txn) (*flow.RichProtocolStateEntry, error) {
		val, err := s.cache.Get(protocolStateID)(tx)
		if err != nil {
			return nil, err
		}
		return val.(*flow.RichProtocolStateEntry), nil
	}
}

func (s *ProtocolState) byBlockID(blockID flow.Identifier) func(*badger.Txn) (*flow.RichProtocolStateEntry, error) {
	return func(tx *badger.Txn) (*flow.RichProtocolStateEntry, error) {
		var protocolStateID flow.Identifier
		err := operation.LookupProtocolState(blockID, &protocolStateID)(tx)
		if err != nil {
			return nil, fmt.Errorf("could not lookup protocol state ID for block (%x): %w", blockID[:], err)
		}
		return s.byID(protocolStateID)(tx)
	}
}

func newRichProtocolStateEntry(protocolState flow.ProtocolStateEntry,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
) (*flow.RichProtocolStateEntry, error) {
	result := &flow.RichProtocolStateEntry{
		ProtocolStateEntry: protocolState,
	}

	// query and fill in epoch setups and commits for previous and current epochs
	var err error
	result.PreviousEpochSetup, err = setups.ByID(protocolState.PreviousEpochEventIDs.SetupID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve previous epoch setup: %w", err)
	}
	result.PreviousEpochCommit, err = commits.ByID(protocolState.PreviousEpochEventIDs.CommitID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve previous epoch commit: %w", err)
	}
	result.CurrentEpochSetup, err = setups.ByID(protocolState.CurrentEpochEventIDs.SetupID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve current epoch setup: %w", err)
	}
	result.CurrentEpochCommit, err = commits.ByID(protocolState.CurrentEpochEventIDs.CommitID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve current epoch commit: %w", err)
	}
	result.Identities, err = buildIdentityTable(protocolState.Identities, result.PreviousEpochSetup, result.CurrentEpochSetup)
	if err != nil {
		return nil, fmt.Errorf("could not build identity table: %w", err)
	}

	// if next epoch has been already committed, fill in data for it as well.
	if protocolState.NextEpochProtocolState != nil {
		nextEpochProtocolState := *protocolState.NextEpochProtocolState

		nextEpochSetup, err := setups.ByID(nextEpochProtocolState.CurrentEpochEventIDs.SetupID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve next epoch setup: %w", err)
		}
		nextEpochCommit, err := commits.ByID(nextEpochProtocolState.CurrentEpochEventIDs.CommitID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve next epoch commit: %w", err)
		}
		nextEpochIdentityTable, err := buildIdentityTable(protocolState.Identities, result.CurrentEpochSetup, nextEpochSetup)
		if err != nil {
			return nil, fmt.Errorf("could not build next epoch identity table: %w", err)
		}

		// fill identities for next epoch
		result.NextEpochProtocolState = &flow.RichProtocolStateEntry{
			ProtocolStateEntry:     nextEpochProtocolState,
			CurrentEpochSetup:      nextEpochSetup,
			CurrentEpochCommit:     nextEpochCommit,
			PreviousEpochSetup:     result.CurrentEpochSetup,  // previous epoch setup is current epoch setup
			PreviousEpochCommit:    result.CurrentEpochCommit, // previous epoch setup is current epoch setup
			Identities:             nextEpochIdentityTable,
			NextEpochProtocolState: nil, // always nil
		}
	}

	return result, nil
}

func buildIdentityTable(
	dynamicIdentities flow.DynamicIdentityEntryList,
	previousEpochSetup, currentEpochSetup *flow.EpochSetup,
) (flow.IdentityList, error) {
	allEpochParticipants := append(previousEpochSetup.Participants, currentEpochSetup.Participants...)
	allEpochParticipants.Sort(order.Canonical)
	// sanity check: size of identities should be equal to previous and current epoch participants combined
	if len(allEpochParticipants) != len(dynamicIdentities) {
		return nil, fmt.Errorf("invalid number of identities in protocol state: expected %d, got %d", len(allEpochParticipants), len(dynamicIdentities))
	}

	// build full identity table for current epoch
	var result flow.IdentityList
	for i, identity := range dynamicIdentities {
		// sanity check: identities should be sorted in canonical order
		if identity.NodeID != allEpochParticipants[i].NodeID {
			return nil, fmt.Errorf("identites in protocol state are not in canonical order: expected %s, got %s", allEpochParticipants[i].NodeID, identity.NodeID)
		}
		result = append(result, &flow.Identity{
			IdentitySkeleton: allEpochParticipants[i].IdentitySkeleton,
			DynamicIdentity:  identity.Dynamic,
		})
	}
	return result, nil
}
