package complete

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/ledger"
	"github.com/dapperlabs/flow-go/ledger/common/encoding"
	"github.com/dapperlabs/flow-go/ledger/common/pathfinder"
	"github.com/dapperlabs/flow-go/ledger/complete/mtrie"
	"github.com/dapperlabs/flow-go/ledger/complete/mtrie/trie"
	"github.com/dapperlabs/flow-go/ledger/complete/wal"
	"github.com/dapperlabs/flow-go/module"
)

// Ledger (complete) is a fast memory-efficient fork-aware thread-safe trie-based key/value storage.
// Ledger holds an array of registers (key-value pairs) and keeps tracks of changes over a limited time.
// Each register is referenced by an ID (key) and holds a value (byte slice).
// Ledger provides atomic batched updates and read (with or without proofs) operation given a list of keys.
// Every update to the Ledger creates a new state which captures the state of the storage.
// Under the hood, it uses binary Merkle tries to generate inclusion and non-inclusion proofs.
// Ledger is fork-aware which means any update can be applied at any previous state which forms a tree of tries (forest).
// The forest is in memory but all changes (e.g. register updates) are captured inside write-ahead-logs for crash recovery reasons.
// In order to limit the memory usage and maintain the performance storage only keeps a limited number of
// tries and purge the old ones (LRU-based); in other words, Ledger is not designed to be used
// for archival usage but make it possible for other software components to reconstruct very old tries using write-ahead logs.
type Ledger struct {
	forest  *mtrie.Forest
	wal     *wal.LedgerWAL
	metrics module.LedgerMetrics
	logger  zerolog.Logger
}

// NewLedger creates a new in-memory trie-backed ledger storage with persistence.
func NewLedger(dbDir string,
	capacity int,
	metrics module.LedgerMetrics,
	log zerolog.Logger,
	reg prometheus.Registerer) (*Ledger, error) {

	w, err := wal.NewWAL(nil, reg, dbDir, capacity, pathfinder.PathByteSize, wal.SegmentSize)
	if err != nil {
		return nil, fmt.Errorf("cannot create LedgerWAL: %w", err)
	}

	forest, err := mtrie.NewForest(pathfinder.PathByteSize, dbDir, capacity, metrics, func(evictedTrie *trie.MTrie) error {
		return w.RecordDelete(evictedTrie.RootHash())
	})
	if err != nil {
		return nil, fmt.Errorf("cannot create forest: %w", err)
	}

	logger := log.With().Str("ledger", "complete").Logger()

	storage := &Ledger{
		forest:  forest,
		wal:     w,
		metrics: metrics,
		logger:  logger,
	}

	err = w.ReplayOnForest(forest)
	if err != nil {
		return nil, fmt.Errorf("cannot restore LedgerWAL: %w", err)
	}

	// TODO update to proper value once https://github.com/dapperlabs/flow-go/pull/3720 is merged
	metrics.ForestApproxMemorySize(0)

	return storage, nil
}

// Ready implements interface module.ReadyDoneAware
// it starts the EventLoop's internal processing loop.
func (l *Ledger) Ready() <-chan struct{} {
	ready := make(chan struct{})
	close(ready)
	return ready
}

// Done implements interface module.ReadyDoneAware
// it closes all the open write-ahead log files.
func (l *Ledger) Done() <-chan struct{} {
	_ = l.wal.Close()
	done := make(chan struct{})
	close(done)
	return done
}

// InitState returns the state of an empty ledger
func (l *Ledger) InitState() ledger.State {
	return ledger.State(l.forest.GetEmptyRootHash())
}

// Get read the values of the given keys at the given state
// it returns the values in the same order as given registerIDs and errors (if any)
func (l *Ledger) Get(query *ledger.Query) (values []ledger.Value, err error) {
	start := time.Now()
	paths, err := pathfinder.KeysToPaths(query.Keys(), 0)
	if err != nil {
		return nil, err
	}
	trieRead := &ledger.TrieRead{RootHash: ledger.RootHash(query.State()), Paths: paths}
	payloads, err := l.forest.Read(trieRead)
	if err != nil {
		return nil, err
	}
	values, err = pathfinder.PayloadsToValues(payloads)
	if err != nil {
		return nil, err
	}

	l.metrics.ReadValuesNumber(uint64(len(paths)))
	readDuration := time.Since(start)
	l.metrics.ReadDuration(readDuration)

	if len(paths) > 0 {
		durationPerValue := time.Duration(readDuration.Nanoseconds()/int64(len(paths))) * time.Nanosecond
		l.metrics.ReadDurationPerItem(durationPerValue)
	}

	return values, err
}

// Set updates the ledger given an update
// it returns the state after update and errors (if any)
func (l *Ledger) Set(update *ledger.Update) (newState ledger.State, err error) {
	start := time.Now()

	// TODO: add test case
	if update.Size() == 0 {
		// return current state root unchanged
		return update.State(), nil
	}

	trieUpdate, err := pathfinder.UpdateToTrieUpdate(update, 0)
	if err != nil {
		return nil, err
	}

	l.metrics.UpdateCount()
	l.metrics.UpdateValuesNumber(uint64(len(trieUpdate.Paths)))

	err = l.wal.RecordUpdate(trieUpdate)
	if err != nil {
		return nil, fmt.Errorf("cannot update state, error while writing LedgerWAL: %w", err)
	}

	newRootHash, err := l.forest.Update(trieUpdate)
	if err != nil {
		return nil, fmt.Errorf("cannot update state: %w", err)
	}

	// TODO update to proper value once https://github.com/dapperlabs/flow-go/pull/3720 is merged
	l.metrics.ForestApproxMemorySize(0)

	elapsed := time.Since(start)
	l.metrics.UpdateDuration(elapsed)

	if len(trieUpdate.Paths) > 0 {
		durationPerValue := time.Duration(elapsed.Nanoseconds()/int64(len(trieUpdate.Paths))) * time.Nanosecond
		l.metrics.UpdateDurationPerItem(durationPerValue)
	}

	l.logger.Info().Hex("from", update.State()).
		Hex("to", newRootHash[:]).
		Int("update_size", update.Size()).
		Msg("ledger updated")
	return ledger.State(newRootHash), nil
}

// Prove provides proofs for a ledger query and errors (if any)
func (l *Ledger) Prove(query *ledger.Query) (proof ledger.Proof, err error) {

	paths, err := pathfinder.KeysToPaths(query.Keys(), 0)
	if err != nil {
		return nil, err
	}

	trieRead := &ledger.TrieRead{RootHash: ledger.RootHash(query.State()), Paths: paths}
	batchProof, err := l.forest.Proofs(trieRead)
	if err != nil {
		return nil, fmt.Errorf("could not get proofs: %w", err)
	}

	proofToGo := encoding.EncodeTrieBatchProof(batchProof)

	if len(paths) > 0 {
		l.metrics.ProofSize(uint32(len(proofToGo) / len(paths)))
	}

	return ledger.Proof(proofToGo), err
}

// CloseStorage closes the DB
func (l *Ledger) CloseStorage() {
	_ = l.wal.Close()
}

// MemSize return the amount of memory used by ledger
// TODO implement an approximate MemSize method
func (l *Ledger) MemSize() (int64, error) {
	return 0, nil
}

// DiskSize returns the amount of disk space used by the storage (in bytes)
func (l *Ledger) DiskSize() (int64, error) {
	return l.forest.DiskSize()
}

// ForestSize returns the number of tries stored in the forest
func (l *Ledger) ForestSize() int {
	return l.forest.Size()
}

// Checkpointer returns a checkpointer instance
func (l *Ledger) Checkpointer() (*wal.Checkpointer, error) {
	checkpointer, err := l.wal.NewCheckpointer()
	if err != nil {
		return nil, fmt.Errorf("cannot create checkpointer for compactor: %w", err)
	}
	return checkpointer, nil
}
