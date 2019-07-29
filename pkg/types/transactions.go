package types

import (
	"time"

	crypto "github.com/dapperlabs/bamboo-node/pkg/crypto/oldcrypto"
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
	Script       []byte
	Nonce        uint64
	ComputeLimit uint64
	Timestamp    time.Time
}

// Hash computes the hash over the necessary transaction data.
func (tx *RawTransaction) Hash() crypto.Hash {
	bytes := crypto.EncodeAsBytes(
		tx.Script,
		tx.Nonce,
		tx.ComputeLimit,
		tx.Timestamp,
	)
	return crypto.NewHash(bytes)
}

// Sign signs a transaction with the given account and keypair.
//
// The function returns a new SignedTransaction that includes the generated signature.
func (tx *RawTransaction) Sign(account Address, keyPair *crypto.KeyPair) *SignedTransaction {
	hash := tx.Hash()

	// TODO: include account in signature
	sig := crypto.Sign(hash, keyPair)

	accountSig := AccountSignature{
		Account:   account,
		PublicKey: keyPair.PublicKey,
		Signature: sig.Bytes(),
	}

	return &SignedTransaction{
		Script:         tx.Script,
		Nonce:          tx.Nonce,
		ComputeLimit:   tx.ComputeLimit,
		Timestamp:      tx.Timestamp,
		PayerSignature: accountSig,
	}
}

// SignedTransaction is a transaction that has been signed by at least one account.
type SignedTransaction struct {
	Script         []byte
	Nonce          uint64
	ComputeLimit   uint64
	ComputeUsed    uint64
	Timestamp      time.Time
	PayerSignature AccountSignature
	Status         TransactionStatus
}

// Hash computes the hash over the necessary transaction data.
func (tx *SignedTransaction) Hash() crypto.Hash {
	bytes := crypto.EncodeAsBytes(
		tx.Script,
		tx.Nonce,
		tx.ComputeLimit,
		tx.Timestamp,
		tx.PayerSignature.Bytes(),
	)
	return crypto.NewHash(bytes)
}
