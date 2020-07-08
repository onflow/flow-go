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
	EmptyStateCommitment() flow.StateCommitment

	// Get returns payload by payload ID at specific stateCommitment
	Get(pID PayloadID, stcom StateCommitment) (payload Payload, err error)

	// Get returns payloads for the given payload IDs at specific stateCommitment
	BatchGet(pID []PayloadID, stcom StateCommitment) (payloads []Payload, err error)

	// Get returns payload by payload ID at specific stateCommitment
	Update(payload Payload, stcom StateCommitment) (newStateCommitment StateCommitment, err error)

	// Get returns payload by payload ID at specific stateCommitment
	BatchUpdate(payloads []Payload, stcom StateCommitment) (newStateCommitment StateCommitment, err error)

	// TODO Proof and Batch Proof

	// TODO Size
	DiskSize() (int64, error)
}
