package types

// RegisterID (key part of key value)
type RegisterID [32]byte

// RegisterValue (value part of Register)
type RegisterValue [32]byte

// Register (key value pairs)
type Register struct {
	ID    *RegisterID
	Value *RegisterValue
}

// RegisterUpdate covers both first time value setting and change value
type RegisterUpdate struct {
	ID       *RegisterID
	NewValue *RegisterValue
}

type StorageProof []byte
type StateCommitment []byte

// Storage takes care of storing registers (key, value pairs) providing proof of correctness
// we aim to store a state of the order of 10^10 registers with up to 1M historic state versions
type Storage interface {
	// GetStateCommitment returns a byte array represnting the current state of the storage (e.g. Merkle root)
	GetStateCommitment() (currentStateCommitment StateCommitment, err error)
	// Batched queries
	GetRegistersWithProofs(registerIds []RegisterID) (registers []*Register, proof StorageProof, err error)
	// Batched query at specific state (historic query)
	GetRegistersWithProofsByStateCommitment(registerIds []RegisterID, stateCommitment StateCommitment) (registers []*Register, proof StorageProof, err error)
	// Batched atomic updates of a subset of registers
	UpdateRegisters(updates []RegisterUpdate) (newStateCommitment StateCommitment, proof StorageProof, err error)
	// Batched atomic updates of a subset of registers at specific state
	UpdateRegistersByStateCommitment(registerIds []RegisterID, stateCommitment StateCommitment) (newStateCommitment StateCommitment, proof StorageProof, err error)
	// Batched queries (without proof)
	GetRegisters(registerIds []RegisterID) (registers []*Register, err error)
	// Batched query at specific state (historic query)
	GetRegistersByStateCommitment(registerIds []RegisterID, stateCommitment StateCommitment) (registers []*Register, err error)
}

// StorageVerifier should be designed as an standalone package to verify proofs of storage
type StorageVerifier interface {
	// verify if a provided proof for getRegisters is accurate
	VerifyGetRegistersProof(registers []Register, proof StorageProof, stateCommitment StateCommitment) (verified bool, err error)
	// verify if a provided proof updateRegister is accurate
	VerifyUpdateRegistersProof(updates []RegisterUpdate, proof StorageProof, initialStateCommitment StateCommitment, finalStateCommitment StateCommitment) (verified bool, err error)
}
