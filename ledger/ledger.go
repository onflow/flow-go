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

	// Get returns payloads for the given payload IDs at specific stateCommitment
	BatchGet(keys []Key, stcom StateCommitment) (values []Value, err error)

	// Get returns payload by payload ID at specific stateCommitment
	Update(key *Key, value *Value, stcom StateCommitment) (newStateCommitment StateCommitment, err error)

	// Get returns payload by payload ID at specific stateCommitment
	BatchUpdate(keys []Key, values []Value, stcom StateCommitment) (newStateCommitment StateCommitment, err error)

	MemSize() (int64, error)

	DiskSize() (int64, error)

	// TODO Proof and Batch Proof
}
