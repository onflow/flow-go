package ledger

import (
	"fmt"
	"sync"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/ledger/trie"
)

type TrieStorage struct {
	lock sync.Mutex
	tree *trie.SMT
	// mForest *mtrie.MForest
}

const (
	Height              = 257
	Interval            = 1000
	CacheSize           = 25      // Max number of open databases
	NumHistoricalStates = 1000000 // EVERYTHING FAILS WHEN THIS LIMIT IS REACHED
)

// NewTrieStorage creates a new trie-backed ledger storage.
func NewTrieStorage(dbDir string) (*TrieStorage, error) {

	tree, err := trie.NewSMT(
		dbDir,
		Height,
		Interval,
		NumHistoricalStates,
		CacheSize,
	)
	if err != nil {
		return nil, err
	}

	// mForest, err := mtrie.NewMForest(257, dbDir, 1000)
	// if err != nil {
	// 	return nil, err
	// }

	return &TrieStorage{
		tree: tree,
		// mForest: mForest,
	}, nil
}

func (f *TrieStorage) Ready() <-chan struct{} {
	ready := make(chan struct{})
	close(ready)
	return ready
}

func (f *TrieStorage) Done() <-chan struct{} {
	f.tree.SafeClose()
	done := make(chan struct{})
	close(done)
	return done
}

func (f *TrieStorage) EmptyStateCommitment() flow.StateCommitment {
	// return f.mForest.GetEmptyRootHash()
	return trie.GetDefaultHashForHeight(f.tree.GetHeight() - 1)
}

// GetRegisters read the values at the given registers at the given flow.StateCommitment
// This is trusted so no proof is generated
func (f *TrieStorage) GetRegisters(
	registerIDs []flow.RegisterID,
	stateCommitment flow.StateCommitment,
) (
	values []flow.RegisterValue,
	err error,
) {
	f.lock.Lock()
	defer f.lock.Unlock()
	values, _, err = f.tree.Read(registerIDs, true, stateCommitment)
	return values, err
}

// UpdateRegisters updates the values at the given registers
// This is trusted so no proof is generated
func (f *TrieStorage) UpdateRegisters(
	ids []flow.RegisterID,
	values []flow.RegisterValue,
	stateCommitment flow.StateCommitment,
) (
	newStateCommitment flow.StateCommitment,
	err error,
) {
	f.lock.Lock()
	defer f.lock.Unlock()

	return f.updateRegisters(ids, values, stateCommitment)
}

func (f *TrieStorage) updateRegisters(
	ids []flow.RegisterID,
	values []flow.RegisterValue,
	stateCommitment flow.StateCommitment,
) (
	newStateCommitment flow.StateCommitment,
	err error,
) {
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

	// newStateCommitment, err = f.mForest.Update(ids, values, stateCommitment)
	newStateCommitment, err = f.tree.Update(ids, values, stateCommitment)
	if err != nil {
		return nil, fmt.Errorf("cannot update state: %w", err)
	}

	return newStateCommitment, nil
}

// GetRegistersWithProof read the values at the given registers at the given flow.StateCommitment
// This is untrusted so a proof is generated
func (f *TrieStorage) GetRegistersWithProof(
	registerIDs []flow.RegisterID,
	stateCommitment flow.StateCommitment,
) (
	values []flow.RegisterValue,
	proofs []flow.StorageProof,
	err error,
) {
	f.lock.Lock()
	defer f.lock.Unlock()

	return f.getRegistersWithProof(registerIDs, stateCommitment)
}

func (f *TrieStorage) getRegistersWithProof(
	registerIDs []flow.RegisterID,
	stateCommitment flow.StateCommitment,
) (
	values []flow.RegisterValue,
	proofs []flow.StorageProof,
	err error,
) {
	values, _, err = f.tree.Read(registerIDs, true, stateCommitment)
	if err != nil {
		return nil, nil, fmt.Errorf("Could not get register values: %w", err)
	}

	// batchProof, err := f.mForest.Proofs(registerIDs, stateCommitment)
	batchProof, err := f.tree.GetBatchProof(registerIDs, stateCommitment)
	if err != nil {
		return nil, nil, fmt.Errorf("Could not get proofs: %w", err)
	}

	// proofToGo := mtrie.EncodeBatchProof(batchProof)
	proofToGo := trie.EncodeProof(batchProof)
	return values, proofToGo, err
}

func (f *TrieStorage) GetRegisterTouches(
	registerIDs []flow.RegisterID,
	stateCommitment flow.StateCommitment,
) (
	[]flow.RegisterTouch,
	error,
) {
	f.lock.Lock()
	defer f.lock.Unlock()

	values, proofs, err := f.getRegistersWithProof(registerIDs, stateCommitment)
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

// UpdateRegistersWithProof updates the values at the given registers
// This is untrusted so a proof is generated
func (f *TrieStorage) UpdateRegistersWithProof(
	ids []flow.RegisterID,
	values []flow.RegisterValue,
	stateCommitment flow.StateCommitment,
) (
	newStateCommitment flow.StateCommitment,
	proofs []flow.StorageProof,
	err error,
) {
	f.lock.Lock()
	defer f.lock.Unlock()

	newStateCommitment, err = f.updateRegisters(ids, values, stateCommitment)
	if err != nil {
		return nil, nil, err
	}

	_, proofs, err = f.getRegistersWithProof(ids, newStateCommitment)
	return newStateCommitment, proofs, err
}

// CloseStorage closes the DB
func (f *TrieStorage) CloseStorage() {
	f.tree.SafeClose()
}

func (f *TrieStorage) Size() (int64, error) {
	return f.tree.Size()
}
