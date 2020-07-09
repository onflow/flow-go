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

	// Get returns value by key at specific stateCommitment
	Get(key *Key, stcom StateCommitment) (value *Value, err error)

	// BatchGet returns payloads for the given payload IDs at specific stateCommitment
	BatchGet(keys []Key, stcom StateCommitment) (values []Value, err error)

	// Update updates the key with a new value at specific stateCommitment
	Update(key *Key, value *Value, stcom StateCommitment) (newStateCommitment StateCommitment, err error)

	// BatchUpdate updates a list of keys with new values at specific stateCommitment
	BatchUpdate(keys []Key, values []Value, stcom StateCommitment) (newStateCommitment StateCommitment, err error)

	// Proof returns a proof for the given key at specific stateCommitment
	Proof(key *Key, stcom StateCommitment) (proof *Proof, err error)

	// Proof returns a batch proof for a list of keys at specific stateCommitment
	BatchProof(keys []Key, stcom StateCommitment) (batchproof *BatchProof, err error)

	// returns an approximate size of memory used by the ledger
	MemSize() (int64, error)

	// returns an approximate size of disk used by the ledger
	DiskSize() (int64, error)
}
