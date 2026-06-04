package complete

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete/payloadless"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

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
// PayloadlessLedger is currently in-memory only; it does not persist updates
// to a write-ahead log.
type PayloadlessLedger struct {
	forest            *payloadless.Forest
	metrics           module.LedgerMetrics
	logger            zerolog.Logger
	pathFinderVersion uint8
}

// NewPayloadlessLedger creates a new in-memory payloadless trie-backed ledger.
//
// `capacity` bounds the number of tries kept in the forest; the least-recently
// added trie is evicted once capacity is exceeded.
func NewPayloadlessLedger(
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

	return &PayloadlessLedger{
		forest:            forest,
		metrics:           metrics,
		logger:            logger,
		pathFinderVersion: pathFinderVer,
	}, nil
}

// Ready implements module.ReadyDoneAware. The payloadless ledger has no
// asynchronous initialization, so the returned channel is already closed.
func (l *PayloadlessLedger) Ready() <-chan struct{} {
	ready := make(chan struct{})
	close(ready)
	return ready
}

// Done implements module.ReadyDoneAware. The payloadless ledger has no
// background workers, so the returned channel is already closed.
func (l *PayloadlessLedger) Done() <-chan struct{} {
	done := make(chan struct{})
	close(done)
	return done
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

	newTrie, err := l.forest.NewTrie(trieUpdate)
	if err != nil {
		return ledger.State(hash.DummyHash), nil, fmt.Errorf("cannot update state: %w", err)
	}

	err = l.forest.AddTrie(newTrie)
	if err != nil {
		return ledger.State(hash.DummyHash), nil, fmt.Errorf("failed to add new trie to forest: %w", err)
	}

	newState = ledger.State(newTrie.RootHash())

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
