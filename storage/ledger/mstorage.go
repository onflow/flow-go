package ledger

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/sequencer"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/proof"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/trie"
	"github.com/dapperlabs/flow-go/storage/ledger/wal"
)

// MTrieStorage is a fast memory-efficient fork-aware thread-safe trie-based key/value storage.
// MTrieStorage holds an array of registers (key value pairs) and keep tracks of changes over a limited time.
// Each register is referenced by an ID (key) and holds a value (byte slice).
// MTrieStorage provides atomic batched update and read (with or without proofs) operation given a list of keys.
// Every update to the MTrieStorage creates a new state commitment which captures the state of the storage.
// Under the hood, it uses binary merkle tries to generate inclusion and noninclusion proofs.
// MTrieStorage is fork-aware that means any update can applied at any previous statecommitments which forms a tree of tries (forest).
// The forrest is in memory but all changes (e.g. register updates) are captured inside write-ahead-logs for crash recovery reasons.
// In order to limit the memory usage and maintain the performance storage only keeps limited number of
// tries and purge the old ones (LRU-based); in other words MTrieStorage is not designed to be used
// for archival use but make it possible for other software components to reconstruct very old tries using write-ahead logs.
type MTrieStorage struct {
	mForest *mtrie.MForest
	wal     *wal.LedgerWAL
	metrics module.LedgerMetrics
}

var maxHeight = 257

// NewMTrieStorage creates a new in-memory trie-backed ledger storage with persistence.
func NewMTrieStorage(dbDir string, cacheSize int, metrics module.LedgerMetrics, reg prometheus.Registerer) (*MTrieStorage, error) {

	w, err := wal.NewWAL(nil, reg, dbDir, cacheSize, maxHeight)

	if err != nil {
		return nil, fmt.Errorf("cannot create LedgerWAL: %w", err)
	}

	mForest, err := mtrie.NewMForest(maxHeight, dbDir, cacheSize, metrics, func(evictedTrie *trie.MTrie) error {
		return w.RecordDelete(evictedTrie.RootHash())
	})
	if err != nil {
		return nil, fmt.Errorf("cannot create MForest: %w", err)
	}
	storage := &MTrieStorage{
		mForest: mForest,
		wal:     w,
		metrics: metrics,
	}

	err = w.Replay(
		func(storableNodes []*sequencer.StorableNode, storableTries []*sequencer.StorableTrie) error {
			forestSequencing := &sequencer.MForestSequencing{Nodes: storableNodes, Tries: storableTries}
			rebuiltTries, err := sequencer.RebuildTries(forestSequencing)
			if err != nil {
				return fmt.Errorf("rebuilding forest from sequenced nodes failed: %w", err)
			}
			err = storage.mForest.AddTries(rebuiltTries)
			if err != nil {
				return fmt.Errorf("adding rebuilt tries to forest failed: %w", err)
			}
			return nil
		},
		func(stateCommitment flow.StateCommitment, keys [][]byte, values [][]byte) error {
			_, err = storage.mForest.Update(stateCommitment, keys, values)
			// _, err := trie.UpdateRegisters(keys, values, stateCommitment)
			return err
		},
		func(stateCommitment flow.StateCommitment) error {
			storage.mForest.RemoveTrie(stateCommitment)
			return nil
		},
	)

	if err != nil {
		return nil, fmt.Errorf("cannot restore LedgerWAL: %w", err)
	}

	// TODO update to proper value once https://github.com/dapperlabs/flow-go/pull/3720 is merged
	metrics.ForestApproxMemorySize(0)

	return storage, nil
}

// Ready implements interface module.ReadyDoneAware
// it starts the EventLoop's internal processing loop.
func (f *MTrieStorage) Ready() <-chan struct{} {
	ready := make(chan struct{})
	close(ready)
	return ready
}

// Done implements interface module.ReadyDoneAware
// it closes all the open write-ahead log files.
func (f *MTrieStorage) Done() <-chan struct{} {
	_ = f.wal.Close()
	done := make(chan struct{})
	close(done)
	return done
}

// EmptyStateCommitment returns the state commitment of an empty store (initial state)
func (f *MTrieStorage) EmptyStateCommitment() flow.StateCommitment {
	return f.mForest.GetEmptyRootHash()
}

// GetRegisters read the values of the given register IDs at the given state commitment
// it returns the values in the same order as given registerIDs and errors (if any)
func (f *MTrieStorage) GetRegisters(
	registerIDs []flow.RegisterID,
	stateCommitment flow.StateCommitment,
) (
	values []flow.RegisterValue,
	err error,
) {
	start := time.Now()
	values, err = f.mForest.Read(stateCommitment, registerIDs)

	f.metrics.ReadValuesNumber(uint64(len(registerIDs)))
	readDuration := time.Since(start)
	f.metrics.ReadDuration(readDuration)

	if len(registerIDs) > 0 {
		durationPerValue := time.Duration(readDuration.Nanoseconds()/int64(len(registerIDs))) * time.Nanosecond
		f.metrics.ReadDurationPerItem(durationPerValue)
	}

	return values, err
}

// UpdateRegisters updates the values by register ID given the state commitment
// it returns a new state commitment (state after update) and errors (if any)
func (f *MTrieStorage) UpdateRegisters(
	ids []flow.RegisterID,
	values []flow.RegisterValue,
	stateCommitment flow.StateCommitment,
) (
	newStateCommitment flow.StateCommitment,
	err error,
) {
	start := time.Now()

	// TODO: add test case
	if len(ids) != len(values) {
		return nil, fmt.Errorf(
			"length of IDs [%d] does not match values [%d]", len(ids), len(values),
		)
	}

	// TODO: add test case
	if len(ids) == 0 {
		// return current state root unchanged
		return stateCommitment, nil
	}

	f.metrics.UpdateCount()
	f.metrics.UpdateValuesNumber(uint64(len(ids)))

	err = f.wal.RecordUpdate(stateCommitment, ids, values)
	if err != nil {
		return nil, fmt.Errorf("cannot update state, error while writing LedgerWAL: %w", err)
	}

	newTrie, err := f.mForest.Update(stateCommitment, ids, values)
	newStateCommitment = newTrie.RootHash()
	if err != nil {
		return nil, fmt.Errorf("cannot update state: %w", err)
	}

	// TODO update to proper value once https://github.com/dapperlabs/flow-go/pull/3720 is merged
	f.metrics.ForestApproxMemorySize(0)

	elapsed := time.Since(start)
	f.metrics.UpdateDuration(elapsed)

	if len(ids) > 0 {
		durationPerValue := time.Duration(elapsed.Nanoseconds()/int64(len(ids))) * time.Nanosecond
		f.metrics.UpdateDurationPerItem(durationPerValue)
	}

	return newStateCommitment, nil
}

// GetRegistersWithProof read the values at the given registers at the given state commitment
// it returns values, inclusion proofs and errors (if any)
func (f *MTrieStorage) GetRegistersWithProof(
	registerIDs []flow.RegisterID,
	stateCommitment flow.StateCommitment,
) (
	values []flow.RegisterValue,
	proofs []flow.StorageProof,
	err error,
) {

	values, err = f.GetRegisters(registerIDs, stateCommitment)

	// values, _, err = f.tree.Read(registerIDs, true, stateCommitment)
	if err != nil {
		return nil, nil, fmt.Errorf("Could not get register values: %w", err)
	}

	batchProof, err := f.mForest.Proofs(stateCommitment, registerIDs)
	// batchProof, err := f.tree.GetBatchProof(registerIDs, stateCommitment)
	if err != nil {
		return nil, nil, fmt.Errorf("Could not get proofs: %w", err)
	}

	proofToGo, totalProofLength := proof.EncodeBatchProof(batchProof)

	if len(proofToGo) > 0 {
		f.metrics.ProofSize(uint32(totalProofLength / len(proofToGo)))
	}

	return values, proofToGo, err
}

// GetRegisterTouches reads values and proofs for the given registers and
// returns an slice of register touches
func (f *MTrieStorage) GetRegisterTouches(
	registerIDs []flow.RegisterID,
	stateCommitment flow.StateCommitment,
) (
	[]flow.RegisterTouch,
	error,
) {
	values, proofs, err := f.GetRegistersWithProof(registerIDs, stateCommitment)
	if err != nil {
		return nil, err
	}
	rets := make([]flow.RegisterTouch, 0, len(registerIDs))
	for i, reg := range registerIDs {
		rt := flow.RegisterTouch{
			RegisterID: reg,
			Value:      values[i],
			Proof:      proofs[i],
		}
		rets = append(rets, rt)
	}
	return rets, nil
}

// UpdateRegistersWithProof updates the values at the given registers and
// provides proof for those registers after update
func (f *MTrieStorage) UpdateRegistersWithProof(
	ids []flow.RegisterID,
	values []flow.RegisterValue,
	stateCommitment flow.StateCommitment,
) (
	newStateCommitment flow.StateCommitment,
	proofs []flow.StorageProof,
	err error,
) {
	newStateCommitment, err = f.UpdateRegisters(ids, values, stateCommitment)
	if err != nil {
		return nil, nil, err
	}

	_, proofs, err = f.GetRegistersWithProof(ids, newStateCommitment)
	return newStateCommitment, proofs, err
}

// CloseStorage closes the DB
func (f *MTrieStorage) CloseStorage() {
	_ = f.wal.Close()
}

// DiskSize returns the amount of disk space used by the storage (in bytes)
func (f *MTrieStorage) DiskSize() (int64, error) {
	return f.mForest.DiskSize()
}

// ForestSize returns the number of tries stored in the forest
func (f *MTrieStorage) ForestSize() int {
	return f.mForest.Size()
}

func (f *MTrieStorage) Checkpointer() (*wal.Checkpointer, error) {
	checkpointer, err := f.wal.Checkpointer()
	if err != nil {
		return nil, fmt.Errorf("cannot create checkpointer for compactor: %w", err)
	}
	return checkpointer, nil
}
