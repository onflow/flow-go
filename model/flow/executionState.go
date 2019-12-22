package flow

// RegisterID (key part of key value)
type RegisterID [32]byte

// RegisterValue (value part of Register)
type RegisterValue [32]byte

// StateProof (proof of a read or update to the state, merkel path of some sort)
type StateProof []byte

// StateCommitment holds the root hash of the tree (Snapshot)
type StateCommitment []byte

// ExecutionState takes care of storing registers (key, value pairs) providing proof of correctness
// we aim to store a state of the order of 10^10 registers with up to 1M historic state versions
type ExecutionState interface {
	// Trusted methods (without proof)
	// Get registers at specific StateCommitment by a list of register ids
	GetRegisters(registerIDs []RegisterID, stateCommitment StateCommitment) (values []RegisterValue, err error)
	// Batched atomic updates of a subset of registers at specific state
	UpdateRegister(registerIDs []RegisterID, values []RegisterValue, stateCommitment StateCommitment) (newStateCommitment StateCommitment, err error)

	// Untrusted methods (providing proofs)
	// Get registers at specific StateCommitment by a list of register ids with proofs
	GetRegistersWithProof(registerIDs []RegisterID, stateCommitment StateCommitment) (values []RegisterValue, proofs []StateProof, err error)
	// Batched atomic updates of a subset of registers at specific state with proofs
	UpdateRegisterWithProof(registerIDs []RegisterID, values []RegisterValue, stateCommitment StateCommitment) (newStateCommitment StateCommitment, proofs []StateProof, err error)
}

// ExecutionStateVerifier should be designed as an standalone package to verify proofs of storage
type ExecutionStateVerifier interface {
	// verify if a provided proof for getRegisters is accurate
	VerifyGetRegistersProof(registerIDs []RegisterID, stateCommitment StateCommitment, values []RegisterValue, proof []StateProof) (verified bool, err error)
	// verify if a provided proof updateRegister is accurate
	VerifyUpdateRegistersProof(registerIDs []RegisterID, values []RegisterValue, startStateCommitment StateCommitment, finalStateCommitment StateCommitment, proof []StateProof) (verified bool, err error)
}
