package types

import (
	"github.com/dapperlabs/bamboo-node/internal/emulator/data"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

// TransactionStatus represents the status of a Transaction.
type TransactionStatus int

const (
	// TransactionPending is the status of a pending transaction.
	TransactionPending TransactionStatus = iota
	// TransactionFinalized is the status of a finalized transaction.
	TransactionFinalized
	// TransactionReverted is the status of a reverted transaction.
	TransactionReverted
	// TransactionSealed is the status of a sealed transaction.
	TransactionSealed
)

// String returns the string representation of a transaction status.
func (s TransactionStatus) String() string {
	return [...]string{"PENDING", "FINALIZED", "REVERTED", "SEALED"}[s]
}

// RawTransaction is an unsigned transaction.
type RawTransaction struct {
	ToAddress    crypto.Address
	Script       []byte
	Nonce        uint64
	ComputeLimit uint64
}

// Hash computes the hash over the necessary transaction data.
func (tx *RawTransaction) Hash() crypto.Hash {
	bytes := data.EncodeAsBytes(
		tx.ToAddress,
		tx.Script,
		tx.Nonce,
		tx.ComputeLimit,
	)
	return crypto.NewHash(bytes)
}

// Sign signs a transaction with the given account and keypair.
//
// The function returns a new SignedTransaction that includes the generated signature.
func (tx *RawTransaction) Sign(account crypto.Address, keyPair *crypto.KeyPair) *SignedTransaction {
	hash := tx.Hash()
	sig := crypto.Sign(hash, account, keyPair)

	return &SignedTransaction{
		ToAddress:      tx.ToAddress,
		Script:         tx.Script,
		Nonce:          tx.Nonce,
		ComputeLimit:   tx.ComputeLimit,
		PayerSignature: sig,
	}
}

// SignedTransaction is a transaction that has been signed by at least one account.
type SignedTransaction struct {
	ToAddress      crypto.Address
	Script         []byte
	Nonce          uint64
	ComputeLimit   uint64
	ComputeUsed    uint64
	PayerSignature *crypto.Signature
	Status         TransactionStatus
}

// Hash computes the hash over the necessary transaction data.
func (tx *SignedTransaction) Hash() crypto.Hash {
	bytes := data.EncodeAsBytes(
		tx.ToAddress,
		tx.Script,
		tx.Nonce,
		tx.ComputeLimit,
		tx.PayerSignature,
	)
	return crypto.NewHash(bytes)
}
