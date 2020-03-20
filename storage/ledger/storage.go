package ledger

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/ledger/databases"
	"github.com/dapperlabs/flow-go/storage/ledger/trie"
)

type TrieStorage struct {
	tree *trie.SMT
}

// NewTrieStorage creates a new trie-backed ledger storage.
func NewTrieStorage(db databases.DAL) (*TrieStorage, error) {
	tree, err := trie.NewSMT(
		db,
		257,
		10000000,
		1000,
		1000000,
		1000,
	)
	if err != nil {
		return nil, err
	}

	return &TrieStorage{
		tree: tree,
	}, nil
}

func (f *TrieStorage) LatestStateCommitment() flow.StateCommitment {
	return f.tree.GetRoot().GetValue()
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
	values, _, err = f.tree.Read(registerIDs, true, stateCommitment)
	return values, err
}

// UpdateRegisters updates the values at the given registers
// This is trusted so no proof is generated
func (f *TrieStorage) UpdateRegisters(
	ids []flow.RegisterID,
	values []flow.RegisterValue,
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
		return f.tree.GetRoot().GetValue(), nil
	}

	err = f.tree.Update(ids, values)

	return f.tree.GetRoot().GetValue(), err
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
	values, proofHldr, err := f.tree.Read(registerIDs, false, stateCommitment)
	if err != nil {
		return nil, nil, fmt.Errorf("Could not get registers with proofs: %w", err)
	}
	proofs = trie.EncodeProof(proofHldr)
	return values, proofs, err
}

func (f *TrieStorage) GetRegisterTouches(
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

// UpdateRegistersWithProof updates the values at the given registers
// This is untrusted so a proof is generated
func (f *TrieStorage) UpdateRegistersWithProof(
	ids []flow.RegisterID,
	values []flow.RegisterValue,
) (
	newStateCommitment flow.StateCommitment,
	proofs []flow.StorageProof,
	err error,
) {
	newStateCommitment, err = f.UpdateRegisters(ids, values)
	if err != nil {
		return nil, nil, err
	}

	_, proofs, err = f.GetRegistersWithProof(ids, newStateCommitment)
	return newStateCommitment, proofs, err
}

// CloseStorage closes the DB
func (f *TrieStorage) CloseStorage() (error, error) {
	return f.tree.SafeClose()
}
