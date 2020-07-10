package ledger

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
)

// StateCommitment captures a commitment to an specific state of the ledger
type StateCommitment []byte

// Ledger takes care of storing and reading registers (key, value pairs)
type Ledger interface {
	module.ReadyDoneAware

	InitStateCommitment() flow.StateCommitment

	// Get returns value by key at specific state commitment
	Get(key *Key, stcom StateCommitment) (value *Value, err error)

	// Get returns values for the given keys (same order) at specific state commitment
	BatchGet(keys []Key, stcom StateCommitment) (values []Value, err error)

	// Update updates the key with a new value at specific state commitment
	Update(key *Key, value *Value, stcom StateCommitment) (newStateCommitment StateCommitment, err error)

	// BatchUpdate updates a list of keys with new values at specific state commitment and returns a new state commitment
	BatchUpdate(keys []Key, values []Value, stcom StateCommitment) (newStateCommitment StateCommitment, err error)

	// Proof returns a proof for the given key at specific state commitment
	Proof(key *Key, stcom StateCommitment) (proof *Proof, err error)

	// Proof returns a batch proof for a list of keys at specific stateCommitment
	BatchProof(keys []Key, stcom StateCommitment) (batchproof *BatchProof, err error)

	// returns an approximate size of memory used by the ledger
	MemSize() (int64, error)

	// returns an approximate size of disk used by the ledger
	DiskSize() (int64, error)
}
