package complete

import (
	"fmt"
	"io"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/encoding"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/module"
)

const DefaultCacheSize = 1000
const DefaultPathFinderVersion = 1

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
	forest            *mtrie.Forest
	wal               wal.LedgerWAL
	metrics           module.LedgerMetrics
	logger            zerolog.Logger
	pathFinderVersion uint8
}

// NewLedger creates a new in-memory trie-backed ledger storage with persistence.
func NewLedger(
	wal wal.LedgerWAL,
	capacity int,
	metrics module.LedgerMetrics,
	log zerolog.Logger,
	pathFinderVer uint8) (*Ledger, error) {

	logger := log.With().Str("ledger", "complete").Logger()

	forest, err := mtrie.NewForest(capacity, metrics, func(evictedTrie *trie.MTrie) {
		err := wal.RecordDelete(evictedTrie.RootHash())
		if err != nil {
			logger.Error().Err(err).Msg("failed to save delete record in wal")
		}
	})
	if err != nil {
		return nil, fmt.Errorf("cannot create forest: %w", err)
	}

	storage := &Ledger{
		forest:            forest,
		wal:               wal,
		metrics:           metrics,
		logger:            logger,
		pathFinderVersion: pathFinderVer,
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
	done := make(chan struct{})
	close(done)
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

// Get read the values of the given keys at the given state
// it returns the values in the same order as given registerIDs and errors (if any)
func (l *Ledger) Get(query *ledger.Query) (values []ledger.Value, err error) {
	start := time.Now()
	paths, err := pathfinder.KeysToPaths(query.Keys(), l.pathFinderVersion)
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
func (l *Ledger) Set(update *ledger.Update) (newState ledger.State, trieUpdate *ledger.TrieUpdate, err error) {
	start := time.Now()

	// TODO: add test case
	if update.Size() == 0 {
		// return current state root unchanged
		return update.State(), nil, nil
	}

	trieUpdate, err = pathfinder.UpdateToTrieUpdate(update, l.pathFinderVersion)
	if err != nil {
		return ledger.State(hash.DummyHash), nil, err
	}

	l.metrics.UpdateCount()
	l.metrics.UpdateValuesNumber(uint64(len(trieUpdate.Paths)))

	walChan := make(chan error)

	go func() {
		walChan <- l.wal.RecordUpdate(trieUpdate)
	}()

	newRootHash, err := l.forest.Update(trieUpdate)
	walError := <-walChan

	if err != nil {
		return ledger.State(hash.DummyHash), nil, fmt.Errorf("cannot update state: %w", err)
	}
	if walError != nil {
		return ledger.State(hash.DummyHash), nil, fmt.Errorf("error while writing LedgerWAL: %w", walError)
	}

	// TODO update to proper value once https://github.com/onflow/flow-go/pull/3720 is merged
	l.metrics.ForestApproxMemorySize(0)

	elapsed := time.Since(start)
	l.metrics.UpdateDuration(elapsed)

	if len(trieUpdate.Paths) > 0 {
		durationPerValue := time.Duration(elapsed.Nanoseconds()/int64(len(trieUpdate.Paths))) * time.Nanosecond
		l.metrics.UpdateDurationPerItem(durationPerValue)
	}

	state := update.State()
	l.logger.Info().Hex("from", state[:]).
		Hex("to", newRootHash[:]).
		Int("update_size", update.Size()).
		Msg("ledger updated")
	return ledger.State(newRootHash), trieUpdate, nil
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

	proofToGo := encoding.EncodeTrieBatchProof(batchProof)

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

// Checkpointer returns a checkpointer instance
func (l *Ledger) Checkpointer() (*wal.Checkpointer, error) {
	checkpointer, err := l.wal.NewCheckpointer()
	if err != nil {
		return nil, fmt.Errorf("cannot create checkpointer for compactor: %w", err)
	}
	return checkpointer, nil
}

// ExportCheckpointAt exports a checkpoint at specific state commitment after applying migrations and returns the new state (after migration) and any errors
func (l *Ledger) ExportCheckpointAt(
	state ledger.State,
	migrations []ledger.Migration,
	reporters []ledger.Reporter,
	targetPathFinderVersion uint8,
	outputDir, outputFile string,
) (ledger.State, error) {

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
		return ledger.State(hash.DummyHash),
			fmt.Errorf("cannot get try at the given state commitment: %w", err)
	}

	// clean up tries to release memory
	err = l.keepOnlyOneTrie(state)
	if err != nil {
		return ledger.State(hash.DummyHash),
			fmt.Errorf("failed to clean up tries to reduce memory usage: %w", err)
	}

	// TODO enable validity check of trie
	// only check validity of the trie we are interested in
	// l.logger.Info().Msg("Checking validity of the trie at the given state...")
	// if !t.IsAValidTrie() {
	//	 return nil, fmt.Errorf("trie is not valid: %w", err)
	// }
	// l.logger.Info().Msg("Trie is valid.")

	// get all payloads
	payloads := t.AllPayloads()
	payloadSize := len(payloads)

	// migrate payloads
	for i, migrate := range migrations {
		l.logger.Info().Msgf("migration %d is underway", i)

		start := time.Now()
		payloads, err = migrate(payloads)
		elapsed := time.Since(start)

		if err != nil {
			return ledger.State(hash.DummyHash), fmt.Errorf("error applying migration (%d): %w", i, err)
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

	l.logger.Info().Msgf("constructing a new trie with migrated payloads (count: %d)...", len(payloads))

	// get paths
	paths, err := pathfinder.PathsFromPayloads(payloads, targetPathFinderVersion)
	if err != nil {
		return ledger.State(hash.DummyHash), fmt.Errorf("cannot export checkpoint, can't construct paths: %w", err)
	}

	emptyTrie := trie.NewEmptyMTrie()

	// no need to prune the data since it has already been prunned through migrations
	applyPruning := false
	newTrie, err := trie.NewTrieWithUpdatedRegisters(emptyTrie, paths, payloads, applyPruning)
	if err != nil {
		return ledger.State(hash.DummyHash), fmt.Errorf("constructing updated trie failed: %w", err)
	}

	statecommitment := ledger.State(newTrie.RootHash())

	l.logger.Info().Msgf("successfully built new trie. NEW ROOT STATECOMMIEMENT: %v", statecommitment.String())

	l.logger.Info().Msg("creating a checkpoint for the new trie")

	writer, err := wal.CreateCheckpointWriterForFile(outputDir, outputFile)
	if err != nil {
		return ledger.State(hash.DummyHash), fmt.Errorf("failed to create a checkpoint writer: %w", err)
	}

	l.logger.Info().Msg("storing the checkpoint to the file")

	err = wal.StoreCheckpoint(writer, newTrie)
	if err != nil {
		return ledger.State(hash.DummyHash), fmt.Errorf("failed to store the checkpoint: %w", err)
	}
	writer.Close()

	l.logger.Info().Msgf("checkpoint file successfully stored at: %v %v", outputDir, outputFile)

	l.logger.Info().Msgf("generating reports")

	// run reporters
	for _, reporter := range reporters {
		l.logger.Info().
			Str("name", reporter.Name()).
			Msg("starting reporter")

		start := time.Now()
		err = reporter.Report(payloads)
		elapsed := time.Since(start)

		l.logger.Info().
			Str("timeTaken", elapsed.String()).
			Str("name", reporter.Name()).
			Msg("reporter done")
		if err != nil {
			return ledger.State(hash.DummyHash),
				fmt.Errorf("error running reporter (%s): %w", reporter.Name(), err)
		}
	}

	l.logger.Info().Msgf("all reports genereated")

	return statecommitment, nil
}

// MostRecentTouchedState returns a state which is most recently touched.
func (l *Ledger) MostRecentTouchedState() (ledger.State, error) {
	root, err := l.forest.MostRecentTouchedRootHash()
	return ledger.State(root), err
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

	allTries, err := l.forest.GetTries()
	if err != nil {
		return err
	}

	targetRootHash := ledger.RootHash(state)
	for _, trie := range allTries {
		trieRootHash := trie.RootHash()
		if trieRootHash != targetRootHash {
			l.forest.RemoveTrie(trieRootHash)
		}
	}
	return nil
}
