package ledger

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
)

// Ledger takes care of storing and reading registers (key, value pairs)
type Ledger interface {
	module.ReadyDoneAware

	InitStateCommitment() flow.StateCommitment

	// Get returns values for the given slice of keys at specific state commitment
	Get(read *Read) (values []Value, err error)

	// Update updates a list of keys with new values at specific state commitment (update) and returns a new state commitment
	Set(update *Update) (newStateCommitment StateCommitment, err error)

	// Proof returns a proof for the given keys at specific state commitment
	Proof(read *Read) (proof *Proof, err error)

	// returns an approximate size of memory used by the ledger
	MemSize() (int64, error)

	// returns an approximate size of disk used by the ledger
	DiskSize() (int64, error)
}
