package complete

import (
	"fmt"
	"io"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	realWAL "github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

const (
	DefaultCacheSize          = 1000
	DefaultPathFinderVersion  = 1
	defaultTrieUpdateChanSize = 500
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
// tries and purge the old ones (FIFO-based); in other words, Ledger is not designed to be used
// for archival usage but make it possible for other software components to reconstruct very old tries using write-ahead logs.
type Ledger struct {
	forest            *mtrie.Forest
	wal               realWAL.LedgerWAL
	metrics           module.LedgerMetrics
	logger            zerolog.Logger
	trieUpdateCh      chan *WALTrieUpdate
	pathFinderVersion uint8
}

// NewLedger creates a new in-memory trie-backed ledger storage with persistence.
func NewLedger(
	wal realWAL.LedgerWAL,
	capacity int,
	metrics module.LedgerMetrics,
	log zerolog.Logger,
	pathFinderVer uint8) (*Ledger, error) {

	logger := log.With().Str("ledger_mod", "complete").Logger()

	forest, err := mtrie.NewForest(capacity, metrics, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create forest: %w", err)
	}

	storage := &Ledger{
		forest:            forest,
		wal:               wal,
		metrics:           metrics,
		logger:            logger,
		pathFinderVersion: pathFinderVer,
		trieUpdateCh:      make(chan *WALTrieUpdate, defaultTrieUpdateChanSize),
	}

	// pause records to prevent double logging trie removals
	wal.PauseRecord()
	defer wal.UnpauseRecord()

	err = wal.ReplayOnForest(forest)
	if err != nil {
		return nil, fmt.Errorf("cannot restore LedgerWAL: %w", err)
	}

	wal.UnpauseRecord()

	// TODO update to proper value once https://github.com/onflow/flow-go/pull/3720 is merged
	metrics.ForestApproxMemorySize(0)

	return storage, nil
}

// TrieUpdateChan returns a channel which is used to receive trie updates that needs to be logged in WALs.
// This channel is closed when ledger component shutdowns down.
func (l *Ledger) TrieUpdateChan() <-chan *WALTrieUpdate {
	return l.trieUpdateCh
}

// Ready implements interface module.ReadyDoneAware
// it starts the EventLoop's internal processing loop.
func (l *Ledger) Ready() <-chan struct{} {
	ready := make(chan struct{})
	go func() {
		defer close(ready)
		// Start WAL component.
		<-l.wal.Ready()
	}()
	return ready
}

// Done implements interface module.ReadyDoneAware
func (l *Ledger) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)

		// Ledger is responsible for closing trieUpdateCh channel,
		// so Compactor can drain and process remaining updates.
		close(l.trieUpdateCh)
	}()
	return done
}

// InitialState returns the state of an empty ledger
func (l *Ledger) InitialState() ledger.State {
	return ledger.State(l.forest.GetEmptyRootHash())
}

// ValueSizes read the values of the given keys at the given state.
// It returns value sizes in the same order as given registerIDs and errors (if any)
func (l *Ledger) ValueSizes(query *ledger.Query) (valueSizes []int, err error) {
	start := time.Now()
	paths, err := pathfinder.KeysToPaths(query.Keys(), l.pathFinderVersion)
	if err != nil {
		return nil, err
	}
	trieRead := &ledger.TrieRead{RootHash: ledger.RootHash(query.State()), Paths: paths}
	valueSizes, err = l.forest.ValueSizes(trieRead)
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

	return valueSizes, err
}

// GetSingleValue reads value of a single given key at the given state.
func (l *Ledger) GetSingleValue(query *ledger.QuerySingleValue) (value ledger.Value, err error) {
	start := time.Now()
	path, err := pathfinder.KeyToPath(query.Key(), l.pathFinderVersion)
	if err != nil {
		return nil, err
	}
	trieRead := &ledger.TrieReadSingleValue{RootHash: ledger.RootHash(query.State()), Path: path}
	value, err = l.forest.ReadSingleValue(trieRead)
	if err != nil {
		return nil, err
	}

	l.metrics.ReadValuesNumber(1)
	readDuration := time.Since(start)
	l.metrics.ReadDuration(readDuration)

	durationPerValue := time.Duration(readDuration.Nanoseconds()) * time.Nanosecond
	l.metrics.ReadDurationPerItem(durationPerValue)

	return value, nil
}

// Get read the values of the given keys at the given state
// it returns the values in the same order as given registerIDs and errors (if any)
func (l *Ledger) Get(query *ledger.Query) (values []ledger.Value, err error) {
	start := time.Now()
	paths, err := pathfinder.KeysToPaths(query.Keys(), l.pathFinderVersion)
	if err != nil {
		return nil, err
	}
	trieRead := &ledger.TrieRead{RootHash: ledger.RootHash(query.State()), Paths: paths}
	values, err = l.forest.Read(trieRead)
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

// Set updates the ledger given an update.
// It returns the state after update and errors (if any)
func (l *Ledger) Set(update *ledger.Update) (newState ledger.State, trieUpdate *ledger.TrieUpdate, err error) {
	if update.Size() == 0 {
		return update.State(),
			&ledger.TrieUpdate{
				RootHash: ledger.RootHash(update.State()),
				Paths:    []ledger.Path{},
				Payloads: []*ledger.Payload{},
			},
			nil
	}

	start := time.Now()

	trieUpdate, err = pathfinder.UpdateToTrieUpdate(update, l.pathFinderVersion)
	if err != nil {
		return ledger.State(hash.DummyHash), nil, err
	}

	l.metrics.UpdateCount()

	newState, err = l.set(trieUpdate)
	if err != nil {
		return ledger.State(hash.DummyHash), nil, err
	}

	// TODO update to proper value once https://github.com/onflow/flow-go/pull/3720 is merged
	l.metrics.ForestApproxMemorySize(0)

	elapsed := time.Since(start)
	l.metrics.UpdateDuration(elapsed)

	if len(trieUpdate.Paths) > 0 {
		durationPerValue := time.Duration(elapsed.Nanoseconds() / int64(len(trieUpdate.Paths)))
		l.metrics.UpdateDurationPerItem(durationPerValue)
	}

	state := update.State()
	l.logger.Info().Hex("from", state[:]).
		Hex("to", newState[:]).
		Int("update_size", update.Size()).
		Msg("ledger updated")
	return newState, trieUpdate, nil
}

func (l *Ledger) set(trieUpdate *ledger.TrieUpdate) (newState ledger.State, err error) {

	// resultCh is a buffered channel to receive WAL update result.
	resultCh := make(chan error, 1)

	// trieCh is a buffered channel to send updated trie.
	// trieCh can be closed without sending updated trie to indicate failure to update trie.
	trieCh := make(chan *trie.MTrie, 1)
	defer close(trieCh)

	// There are two goroutines:
	// 1. writing the trie update to WAL (in Compactor goroutine)
	// 2. creating a new trie from the trie update (in this goroutine)
	// Since writing to WAL is running concurrently, we use resultCh
	// to receive WAL update result from Compactor.
	// Compactor also needs new trie created here because Compactor
	// caches new trie to minimize memory foot-print while checkpointing.
	// `trieCh` is used to send created trie to Compactor.
	l.trieUpdateCh <- &WALTrieUpdate{Update: trieUpdate, ResultCh: resultCh, TrieCh: trieCh}

	newTrie, err := l.forest.NewTrie(trieUpdate)
	walError := <-resultCh

	if err != nil {
		return ledger.State(hash.DummyHash), fmt.Errorf("cannot update state: %w", err)
	}
	if walError != nil {
		return ledger.State(hash.DummyHash), fmt.Errorf("error while writing LedgerWAL: %w", walError)
	}

	err = l.forest.AddTrie(newTrie)
	if err != nil {
		return ledger.State(hash.DummyHash), fmt.Errorf("failed to add new trie to forest: %w", err)
	}

	trieCh <- newTrie

	return ledger.State(newTrie.RootHash()), nil
}

// Prove provides proofs for a ledger query and errors (if any).
//
// Proves are generally _not_ provided in the register order of the query.
// In the current implementation, proofs are sorted in a deterministic order specified by the
// forest and mtrie implementation.
func (l *Ledger) Prove(query *ledger.Query) (proof ledger.Proof, err error) {

	paths, err := pathfinder.KeysToPaths(query.Keys(), l.pathFinderVersion)
	if err != nil {
		return nil, err
	}

	trieRead := &ledger.TrieRead{RootHash: ledger.RootHash(query.State()), Paths: paths}
	batchProof, err := l.forest.Proofs(trieRead)
	if err != nil {
		return nil, fmt.Errorf("could not get proofs: %w", err)
	}

	proofToGo := ledger.EncodeTrieBatchProof(batchProof)

	if len(paths) > 0 {
		l.metrics.ProofSize(uint32(len(proofToGo) / len(paths)))
	}

	return proofToGo, err
}

// MemSize return the amount of memory used by ledger
// TODO implement an approximate MemSize method
func (l *Ledger) MemSize() (int64, error) {
	return 0, nil
}

// ForestSize returns the number of tries stored in the forest
func (l *Ledger) ForestSize() int {
	return l.forest.Size()
}

// Tries returns the tries stored in the forest
func (l *Ledger) Tries() ([]*trie.MTrie, error) {
	return l.forest.GetTries()
}

// Checkpointer returns a checkpointer instance
func (l *Ledger) Checkpointer() (*realWAL.Checkpointer, error) {
	checkpointer, err := l.wal.NewCheckpointer()
	if err != nil {
		return nil, fmt.Errorf("cannot create checkpointer for compactor: %w", err)
	}
	return checkpointer, nil
}

func (l *Ledger) MigrateAt(
	state ledger.State,
	migrations []ledger.Migration,
	targetPathFinderVersion uint8,
) (*trie.MTrie, error) {
	l.logger.Info().Msgf(
		"Ledger is loaded, checkpoint export has started for state %s, and %d migrations have been planed",
		state.String(),
		len(migrations),
	)

	// get trie
	t, err := l.forest.GetTrie(ledger.RootHash(state))
	if err != nil {
		rh, _ := l.forest.MostRecentTouchedRootHash()
		l.logger.Info().
			Str("hash", rh.String()).
			Msgf("Most recently touched root hash.")
		return nil,
			fmt.Errorf("cannot get trie at the given state commitment: %w", err)
	}

	// clean up tries to release memory
	err = l.keepOnlyOneTrie(state)
	if err != nil {
		return nil,
			fmt.Errorf("failed to clean up tries to reduce memory usage: %w", err)
	}

	var payloads []ledger.Payload
	var newTrie *trie.MTrie

	noMigration := len(migrations) == 0

	if noMigration {
		// when there is no migration, reuse the trie without rebuilding it
		newTrie = t
	} else {
		// get all payloads
		payloads = t.AllPayloads()
		payloadSize := len(payloads)

		// migrate payloads
		for i, migrate := range migrations {
			l.logger.Info().Msgf("migration %d/%d is underway", i, len(migrations))

			start := time.Now()
			payloads, err = migrate(payloads)
			elapsed := time.Since(start)

			if err != nil {
				return nil, fmt.Errorf("error applying migration (%d): %w", i, err)
			}

			newPayloadSize := len(payloads)

			if payloadSize != newPayloadSize {
				l.logger.Warn().
					Int("migration_step", i).
					Int("expected_size", payloadSize).
					Int("outcome_size", newPayloadSize).
					Msg("payload counts has changed during migration, make sure this is expected.")
			}
			l.logger.Info().Str("timeTaken", elapsed.String()).Msgf("migration %d is done", i)

			payloadSize = newPayloadSize
		}

		l.logger.Info().Msgf("creating paths for %v payloads", len(payloads))

		// get paths
		paths, err := pathfinder.PathsFromPayloads(payloads, targetPathFinderVersion)
		if err != nil {
			return nil, fmt.Errorf("cannot export checkpoint, can't construct paths: %w", err)
		}

		l.logger.Info().Msgf("constructing a new trie with migrated payloads (count: %d)...", len(payloads))

		emptyTrie := trie.NewEmptyMTrie()

		// no need to prune the data since it has already been prunned through migrations
		applyPruning := false
		newTrie, _, err = trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads, applyPruning)
		if err != nil {
			return nil, fmt.Errorf("constructing updated trie failed: %w", err)
		}
	}

	statecommitment := ledger.State(newTrie.RootHash())

	l.logger.Info().Msgf("successfully built new trie. NEW ROOT STATECOMMIEMENT: %v", statecommitment.String())

	return newTrie, nil
}

// MostRecentTouchedState returns a state which is most recently touched.
func (l *Ledger) MostRecentTouchedState() (ledger.State, error) {
	root, err := l.forest.MostRecentTouchedRootHash()
	return ledger.State(root), err
}

// HasState returns true if the given state exists inside the ledger
func (l *Ledger) HasState(state ledger.State) bool {
	return l.forest.HasTrie(ledger.RootHash(state))
}

// DumpTrieAsJSON export trie at specific state as JSONL (each line is JSON encoding of a payload)
func (l *Ledger) DumpTrieAsJSON(state ledger.State, writer io.Writer) error {
	fmt.Println(ledger.RootHash(state))
	trie, err := l.forest.GetTrie(ledger.RootHash(state))
	if err != nil {
		return fmt.Errorf("cannot find the target trie: %w", err)
	}
	return trie.DumpAsJSON(writer)
}

// this operation should only be used for exporting
func (l *Ledger) keepOnlyOneTrie(state ledger.State) error {
	// don't write things to WALs
	l.wal.PauseRecord()
	defer l.wal.UnpauseRecord()
	return l.forest.PurgeCacheExcept(ledger.RootHash(state))
}

// FindTrieByStateCommit iterates over the ledger tries and compares the root hash to the state commitment
// if a match is found it is returned, otherwise a nil value is returned indicating no match was found
func (l *Ledger) FindTrieByStateCommit(commitment flow.StateCommitment) (*trie.MTrie, error) {
	tries, err := l.Tries()
	if err != nil {
		return nil, err
	}

	for _, t := range tries {
		if t.RootHash().Equals(ledger.RootHash(commitment)) {
			return t, nil
		}
	}

	return nil, nil
}
