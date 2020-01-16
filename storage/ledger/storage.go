package ledger

import (
	"fmt"
	"github.com/dapperlabs/flow-go/model/flow"

	"github.com/dapperlabs/flow-go/storage/ledger/databases"
	"github.com/dapperlabs/flow-go/storage/ledger/trie"
	"github.com/dapperlabs/flow-go/storage/ledger/utils"
)

type TrieStorage struct {
	tree *trie.SMT
}

// NewTrieStorage creates a new trie-backed ledger storage.
func NewTrieStorage(db databases.DAL) (*TrieStorage, error) {
	tree, err := trie.NewSMT(
		db,
		255,
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
func (f *TrieStorage) GetRegisters(registerIDs []flow.RegisterID, stateCommitment flow.StateCommitment) (values []flow.RegisterValue, err error) {
	values, _, err = f.tree.Read(registerIDs, true, stateCommitment)
	return values, err
}

// UpdateRegisters updates the values at the given registers
// This is trusted so no proof is generated
func (f *TrieStorage) UpdateRegisters(ids [][]byte, values []flow.RegisterValue) (newStateCommitment flow.StateCommitment, err error) {
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
func (f *TrieStorage) GetRegistersWithProof(registerIDs []flow.RegisterID, stateCommitment flow.StateCommitment) (values []flow.RegisterValue, proofs []flow.StorageProof, err error) {
	values, proofHldr, err := f.tree.Read(registerIDs, false, stateCommitment)
	if err != nil {
		return nil, nil, err
	}
	// The following code is the encoding logic
	// Each slice in the proofHolder is stored as a byte array, and the whole thing is stored
	// as a [][]byte
	// First we have a byte, and set the first bit to 1 if it is an inclusion proof
	// Then the size is encoded as a single byte
	// Then the flag is encoded
	// Finally the proofs are encoded one at a time, and is stored as a byte array
	proofs = make([]flow.StorageProof, 0)
	for i := 0; i < proofHldr.GetSize(); i++ {
		flag, singleProof, inclusion, size := proofHldr.ExportProof(i)
		byteSize := []byte{size}
		byteInclusion := make([]byte, 1)
		if inclusion {
			utils.SetBit(byteInclusion, 0)
		}
		proof := append(byteInclusion, byteSize...)
		proof = append(proof, flag...)
		for _, p := range singleProof {
			proof = append(proof, p...)
		}
		// ledgerStorage is a struct that holds our SM
		proofs = append(proofs, proof)
	}

	return values, proofs, err
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

	_, proofs, err = f.GetRegistersWithProof(ids, f.tree.GetRoot().GetValue())
	return newStateCommitment, proofs, err
}

// CloseStorage closes the DB
func (f *TrieStorage) CloseStorage() (error, error) {
	return f.tree.SafeClose()
}
