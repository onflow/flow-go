package complete

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete/payloadless"
	realWAL "github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// WALPayloadlessTrieUpdate is the message sent from [PayloadlessLedger.Set]
// to a payloadless compactor over the trie-update channel. It mirrors
// [WALTrieUpdate] but carries the new *payloadless.MTrie back to the compactor
// on TrieCh so the compactor can enqueue it in its checkpoint queue.
type WALPayloadlessTrieUpdate struct {
	Update   *ledger.TrieUpdate        // update to be encoded into the WAL
	ResultCh chan<- error              // compactor sends back the WAL write result
	TrieCh   <-chan *payloadless.MTrie // ledger sends the freshly-built trie to the compactor
}

// PayloadlessLedger is a fork-aware, in-memory trie-based key/leaf-hash storage.
//
// Unlike [Ledger], the underlying trie does not retain payload values: each leaf
// only retains its hash (HashLeaf(path, value)). Reads therefore return leaf
// hashes rather than the original values. Use this variant when the caller only
// needs commitment-level verification (e.g. payloadless execution) and does not
// need the values themselves.
//
// PayloadlessLedger is fork-aware: any update can be applied at any previous
// state which forms a tree of tries (forest). The forest is kept entirely in
// memory and is bounded by `forestCapacity`. When more tries are added than the
// capacity, the Least Recently Added trie is removed (FIFO).
//
// PayloadlessLedger persists updates to a write-ahead log when constructed
// with a non-nil [realWAL.LedgerWAL]; otherwise it operates purely in-memory.
type PayloadlessLedger struct {
	forest            *payloadless.Forest
	wal               realWAL.LedgerWAL
	metrics           module.LedgerMetrics
	logger            zerolog.Logger
	trieUpdateCh      chan *WALPayloadlessTrieUpdate
	closeTrieUpdateCh sync.Once
	pathFinderVersion uint8
}

// defaultPayloadlessTrieUpdateChanSize matches the V6 ledger's buffer size and
// is shared by [PayloadlessLedger.trieUpdateCh]. Tuned for the same workload
// characteristics — a burst-tolerant buffer between Set and the compactor.
const defaultPayloadlessTrieUpdateChanSize = defaultTrieUpdateChanSize

// NewPayloadlessLedger creates a new payloadless trie-backed ledger.
//
// When `wal` is non-nil the ledger:
//   - serializes each [Set] update through a [WALPayloadlessTrieUpdate] sent
//     over [TrieUpdateChan], blocking until the consumer (typically a
//     [PayloadlessCompactor]) reports the WAL write outcome;
//   - exposes a non-nil channel from [TrieUpdateChan].
//
// When `wal` is nil the ledger is purely in-memory: [Set] applies updates
// synchronously and [TrieUpdateChan] returns nil. This mode is intended for
// tests and short-lived experimental nodes that don't need persistence.
//
// `capacity` bounds the number of tries kept in the forest; the least-recently
// added trie is evicted once capacity is exceeded.
func NewPayloadlessLedger(
	wal realWAL.LedgerWAL,
	capacity int,
	metrics module.LedgerMetrics,
	log zerolog.Logger,
	pathFinderVer uint8,
) (*PayloadlessLedger, error) {

	logger := log.With().Str("ledger_mod", "complete-payloadless").Logger()

	forest, err := payloadless.NewForest(capacity, metrics, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create payloadless forest: %w", err)
	}

	l := &PayloadlessLedger{
		forest:            forest,
		wal:               wal,
		metrics:           metrics,
		logger:            logger,
		pathFinderVersion: pathFinderVer,
	}

	// When a WAL is attached, recover in-memory state from the latest V7
	// checkpoint plus newer WAL segments before serving requests. This mirrors
	// the V6 [NewLedger] recovery via [realWAL.LedgerWAL.ReplayOnForest]. When no
	// WAL is attached the ledger is purely in-memory and there is nothing to
	// recover.
	if wal != nil {
		l.trieUpdateCh = make(chan *WALPayloadlessTrieUpdate, defaultPayloadlessTrieUpdateChanSize)

		// pause records to prevent double logging trie updates during replay
		wal.PauseRecord()
		defer wal.UnpauseRecord()

		err = wal.ReplayOnPayloadlessForest(forest)
		if err != nil {
			return nil, fmt.Errorf("cannot restore LedgerWAL: %w", err)
		}

		wal.UnpauseRecord()
	}
	return l, nil
}

// TrieUpdateChan returns the channel that [Set] uses to publish trie updates
// to the consumer (typically a [PayloadlessCompactor]). Returns nil when the
// ledger was constructed without a WAL — in that case [Set] applies updates
// synchronously.
//
// The returned channel is closed by [PayloadlessLedger.Done] so the consumer
// can drain any in-flight updates.
func (l *PayloadlessLedger) TrieUpdateChan() <-chan *WALPayloadlessTrieUpdate {
	return l.trieUpdateCh
}

// Ready implements module.ReadyDoneAware. When a WAL is attached, Ready
// gates on the WAL's own readiness; otherwise it returns an already-closed
// channel.
func (l *PayloadlessLedger) Ready() <-chan struct{} {
	if l.wal == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	ready := make(chan struct{})
	go func() {
		defer close(ready)
		<-l.wal.Ready()
	}()
	return ready
}

// Done implements module.ReadyDoneAware. When a WAL is attached, Done closes
// the trie-update channel so a compactor can drain pending updates before the
// WAL is shut down. The WAL itself is closed by the compactor (matching the V6
// ordering), so Done returns once channel closure has been signaled.
func (l *PayloadlessLedger) Done() <-chan struct{} {
	if l.trieUpdateCh == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	l.closeTrieUpdateCh.Do(func() {
		close(l.trieUpdateCh)
	})
	ch := make(chan struct{})
	close(ch)
	return ch
}

// InitialState returns the state of an empty ledger.
func (l *PayloadlessLedger) InitialState() ledger.State {
	return ledger.State(l.forest.GetEmptyRootHash())
}

// HasState returns true if the given state exists inside the ledger.
func (l *PayloadlessLedger) HasState(state ledger.State) bool {
	return l.forest.HasTrie(ledger.RootHash(state))
}

// HasPaths reports, for each key in `query`, whether the key has an allocated
// register at the given state. The returned slice is in the same order as
// `query.Keys()`.
//
// HasPaths replaces the full ledger's ValueSizes for payloadless mode, since
// the payloadless trie does not retain payload byte sizes.
func (l *PayloadlessLedger) HasPaths(query *ledger.Query) ([]bool, error) {
	paths, err := pathfinder.KeysToPaths(query.Keys(), l.pathFinderVersion)
	if err != nil {
		return nil, err
	}
	trieRead := &ledger.TrieRead{RootHash: ledger.RootHash(query.State()), Paths: paths}
	return l.forest.HasPaths(trieRead)
}

// GetSingleLeafHash returns the leaf hash (HashLeaf(path, value)) for the
// given key at the given state. Returns nil if the path has no allocated
// register.
//
// GetSingleLeafHash replaces the full ledger's GetSingleValue for payloadless
// mode, since payload values are not retained.
func (l *PayloadlessLedger) GetSingleLeafHash(query *ledger.QuerySingleValue) (*hash.Hash, error) {
	start := time.Now()
	path, err := pathfinder.KeyToPath(query.Key(), l.pathFinderVersion)
	if err != nil {
		return nil, err
	}
	trieRead := &ledger.TrieReadSingleValue{RootHash: ledger.RootHash(query.State()), Path: path}
	leafHash, err := l.forest.ReadSingleLeafHash(trieRead)
	if err != nil {
		return nil, err
	}

	l.metrics.ReadValuesNumber(1)
	readDuration := time.Since(start)
	l.metrics.ReadDuration(readDuration)
	l.metrics.ReadDurationPerItem(readDuration)

	return leafHash, nil
}

// GetLeafHashes returns leaf hashes for the given keys at the given state,
// in the same order as `query.Keys()`. A nil entry indicates the path has no
// allocated register at the given state.
//
// GetLeafHashes replaces the full ledger's Get for payloadless mode, since
// payload values are not retained.
func (l *PayloadlessLedger) GetLeafHashes(query *ledger.Query) ([]*hash.Hash, error) {
	start := time.Now()
	paths, err := pathfinder.KeysToPaths(query.Keys(), l.pathFinderVersion)
	if err != nil {
		return nil, err
	}
	trieRead := &ledger.TrieRead{RootHash: ledger.RootHash(query.State()), Paths: paths}
	leafHashes, err := l.forest.ReadLeafHashes(trieRead)
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

	return leafHashes, nil
}

// Set applies the given update to the ledger and returns the new state and
// the trie update that was applied. The update payload's `value` bytes are
// hashed into the trie; the payload's key is not retained.
//
// When the ledger was constructed with a WAL, Set publishes the trie update on
// [TrieUpdateChan] and waits for the consumer (compactor) to confirm the WAL
// write; the new trie is computed in parallel with the WAL write. When the
// ledger was constructed without a WAL, Set applies the update synchronously.
func (l *PayloadlessLedger) Set(update *ledger.Update) (newState ledger.State, trieUpdate *ledger.TrieUpdate, err error) {
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
		Msg("payloadless ledger updated")
	return newState, trieUpdate, nil
}

// set applies a [ledger.TrieUpdate] to the forest and returns the new root.
//
// If a WAL is attached, set publishes the update on [trieUpdateCh] and waits
// for the compactor's WAL-write outcome on ResultCh; the new trie is computed
// concurrently with the WAL write and handed back to the compactor on TrieCh
// for inclusion in the checkpoint queue. This mirrors the V6 [Ledger.set]
// contract exactly so [TrieUpdateChan] consumers can be uniform across modes.
//
// If no WAL is attached, set applies the update synchronously without any
// channel coordination.
//
// No error returns are expected during normal operation.
func (l *PayloadlessLedger) set(trieUpdate *ledger.TrieUpdate) (ledger.State, error) {
	if l.trieUpdateCh == nil {
		newTrie, err := l.forest.NewTrie(trieUpdate)
		if err != nil {
			return ledger.State(hash.DummyHash), fmt.Errorf("cannot update state: %w", err)
		}
		if err := l.forest.AddTrie(newTrie); err != nil {
			return ledger.State(hash.DummyHash), fmt.Errorf("failed to add new trie to forest: %w", err)
		}
		return ledger.State(newTrie.RootHash()), nil
	}

	// resultCh is a buffered channel to receive the WAL write outcome from the
	// compactor.
	resultCh := make(chan error, 1)
	// trieCh is a buffered channel used to ship the freshly-built trie from this
	// goroutine to the compactor. The compactor stages it into its checkpoint
	// queue. trieCh may be closed without sending when trie construction fails.
	trieCh := make(chan *payloadless.MTrie, 1)
	defer close(trieCh)

	l.trieUpdateCh <- &WALPayloadlessTrieUpdate{Update: trieUpdate, ResultCh: resultCh, TrieCh: trieCh}

	newTrie, err := l.forest.NewTrie(trieUpdate)
	walError := <-resultCh

	if err != nil {
		return ledger.State(hash.DummyHash), fmt.Errorf("cannot update state: %w", err)
	}
	if walError != nil {
		return ledger.State(hash.DummyHash), fmt.Errorf("error while writing LedgerWAL: %w", walError)
	}

	if err := l.forest.AddTrie(newTrie); err != nil {
		return ledger.State(hash.DummyHash), fmt.Errorf("failed to add new trie to forest: %w", err)
	}

	trieCh <- newTrie
	return ledger.State(newTrie.RootHash()), nil
}

// Prove returns a payloadless batch proof for the given keys at the given
// state. The returned proofs carry leaf hashes rather than full payload values.
//
// Proofs are generally _not_ provided in the register order of the query.
// In the current implementation, proofs follow the order specified by the
// underlying payloadless forest implementation.
func (l *PayloadlessLedger) Prove(query *ledger.Query) (*ledger.PayloadlessTrieBatchProof, error) {
	paths, err := pathfinder.KeysToPaths(query.Keys(), l.pathFinderVersion)
	if err != nil {
		return nil, err
	}

	trieRead := &ledger.TrieRead{RootHash: ledger.RootHash(query.State()), Paths: paths}
	batchProof, err := l.forest.Proofs(trieRead)
	if err != nil {
		return nil, fmt.Errorf("could not get proofs: %w", err)
	}

	return batchProof, nil
}

// MemSize returns the amount of memory used by the ledger.
// TODO implement an approximate MemSize method.
func (l *PayloadlessLedger) MemSize() (int64, error) {
	return 0, nil
}

// ForestSize returns the number of tries stored in the forest.
func (l *PayloadlessLedger) ForestSize() int {
	return l.forest.Size()
}

// Tries returns the tries stored in the forest.
func (l *PayloadlessLedger) Tries() ([]*payloadless.MTrie, error) {
	return l.forest.GetTries()
}

// Trie returns the trie stored in the forest with the given root hash.
func (l *PayloadlessLedger) Trie(rootHash ledger.RootHash) (*payloadless.MTrie, error) {
	return l.forest.GetTrie(rootHash)
}

// MostRecentTouchedState returns the state most recently touched.
func (l *PayloadlessLedger) MostRecentTouchedState() (ledger.State, error) {
	root, err := l.forest.MostRecentTouchedRootHash()
	return ledger.State(root), err
}

// FindTrieByStateCommit iterates over the ledger tries and compares the root
// hash to the state commitment. Returns a nil trie if no match is found.
func (l *PayloadlessLedger) FindTrieByStateCommit(commitment flow.StateCommitment) (*payloadless.MTrie, error) {
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

// StateCount returns the number of states (tries) stored in the forest.
func (l *PayloadlessLedger) StateCount() int {
	return l.ForestSize()
}

// StateByIndex returns the state at the given index. `-1` returns the last index.
func (l *PayloadlessLedger) StateByIndex(index int) (ledger.State, error) {
	tries, err := l.Tries()
	if err != nil {
		return ledger.DummyState, fmt.Errorf("failed to get tries: %w", err)
	}

	count := len(tries)
	if count == 0 {
		return ledger.DummyState, fmt.Errorf("no states available")
	}

	if index < 0 {
		index = count + index
		if index < 0 {
			return ledger.DummyState, fmt.Errorf("index %d is out of range (count: %d)", index-count, count)
		}
	}

	if index >= count {
		return ledger.DummyState, fmt.Errorf("index %d is out of range (count: %d)", index, count)
	}

	return ledger.State(tries[index].RootHash()), nil
}
