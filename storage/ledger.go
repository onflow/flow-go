package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
)

// Ledger takes care of storing registers (key, value pairs) providing proof of correctness
// we aim to store a state of the order of 10^10 registers with up to 1M historic state versions
type Ledger interface {
	module.ReadyDoneAware
	EmptyStateCommitment() flow.StateCommitment

	// Trusted methods (without proof)
	// Get registers at specific StateCommitment by a list of register ids
	GetRegisters(registerIDs []flow.RegisterID, stateCommitment flow.StateCommitment) (values []flow.RegisterValue, err error)
	// Batched atomic updates of a subset of registers at specific state
	UpdateRegisters(registerIDs []flow.RegisterID, values []flow.RegisterValue, stateCommitment flow.StateCommitment) (newStateCommitment flow.StateCommitment, err error)

	// Untrusted methods (providing proofs)
	// Get registers at specific StateCommitment by a list of register ids with proofs
	GetRegistersWithProof(registerIDs []flow.RegisterID, stateCommitment flow.StateCommitment) (values []flow.RegisterValue, proofs []flow.StorageProof, err error)
	// Batched atomic updates of a subset of registers at specific state with proofs
	UpdateRegistersWithProof(registerIDs []flow.RegisterID, values []flow.RegisterValue, stateCommitment flow.StateCommitment) (newStateCommitment flow.StateCommitment, proofs []flow.StorageProof, err error)
}

// LedgerVerifier should be designed as an standalone package to verify proofs of storage
type LedgerVerifier interface {
	// verify if a provided proof for getRegisters is accurate
	VerifyRegistersProof(registerIDs []flow.RegisterID, stateCommitment flow.StateCommitment, values []flow.RegisterValue, proof []flow.StorageProof) (verified bool, err error)
}
