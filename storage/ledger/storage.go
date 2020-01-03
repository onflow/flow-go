package ledger

import (
	"github.com/dapperlabs/flow-go/storage/ledger/databases/lvldb"
	"github.com/dapperlabs/flow-go/storage/ledger/trie"
	"github.com/dapperlabs/flow-go/storage/ledger/utils"
)

type TrieStorage struct {
	tree *trie.SMT
}

// NewTrieStorage creates a new trie-backed ledger storage.
func NewTrieStorage(kvdbPath, tdbPath string) (*TrieStorage, error) {
	db, err := lvldb.NewLvlDB(kvdbPath, tdbPath)
	if err != nil {
		return nil, err
	}

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

// GetRegisters read the values at the given registers at the given StateCommitment
// This is trusted so no proof is generated
func (f *TrieStorage) GetRegisters(registerIDs []RegisterID, stateCommitment StateCommitment) (values []RegisterValue, err error) {
	values, _, err = f.tree.Read(registerIDs, true, stateCommitment)
	return values, err
}

// UpdateRegisters updates the values at the given registers
// This is trusted so no proof is generated
func (f *TrieStorage) UpdateRegisters(registerIDs [][]byte, values []RegisterValue) (newStateCommitment StateCommitment, err error) {
	err = f.tree.Update(registerIDs, values)
	return f.tree.GetRoot().GetValue(), err
}

// GetRegistersWithProof read the values at the given registers at the given StateCommitment
// This is untrusted so a proof is generated
func (f *TrieStorage) GetRegistersWithProof(registerIDs []RegisterID, stateCommitment StateCommitment) (values []RegisterValue, proofs []StorageProof, err error) {
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
	proofs = make([]StorageProof, 0)
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
func (f *TrieStorage) UpdateRegistersWithProof(registerIDs []RegisterID, values []RegisterValue) (newStateCommitment StateCommitment, proofs []StorageProof, err error) {
	err = f.tree.Update(registerIDs, values)
	if err != nil {
		return nil, nil, err
	}
	_, proofs, err = f.GetRegistersWithProof(registerIDs, f.tree.GetRoot().GetValue())
	return f.tree.GetRoot().GetValue(), proofs, err

}

// CloseStorage closes the DB
func (f *TrieStorage) CloseStorage() (error, error) {
	return f.tree.SafeClose()
}
