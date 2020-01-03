package ledger

// RegisterID (key part of key value)
type RegisterID = []byte

// RegisterValue (value part of Register)
type RegisterValue = []byte

type StorageProof = []byte
type StateCommitment []byte

// Storage takes care of storing registers (key, value pairs) providing proof of correctness
// we aim to store a state of the order of 10^10 registers with up to 1M historic state versions
type Storage interface {
	// Trusted methods (without proof)
	// Get registers at specific StateCommitment by a list of register ids
	GetRegisters(registerIDs []RegisterID, stateCommitment StateCommitment) (values []RegisterValue, err error)
	// Batched atomic updates of a subset of registers at specific state
	UpdateRegisters(registerIDs []RegisterID, values []RegisterValue) (newStateCommitment StateCommitment, err error)

	// Untrusted methods (providing proofs)
	// Get registers at specific StateCommitment by a list of register ids with proofs
	GetRegistersWithProof(registerIDs []RegisterID, stateCommitment StateCommitment) (values []RegisterValue, proofs []StorageProof, err error)
	// Batched atomic updates of a subset of registers at specific state with proofs
	UpdateRegistersWithProof(registerIDs []RegisterID, values []RegisterValue) (newStateCommitment StateCommitment, proofs []StorageProof, err error)
}

// Verifier should be designed as an standalone package to verify proofs of storage
type Verifier interface {
	// verify if a provided proof for getRegisters is accurate
	VerifyRegistersProof(registerIDs []RegisterID, stateCommitment StateCommitment, values []RegisterValue, proof []StorageProof) (verified bool, err error)
}
