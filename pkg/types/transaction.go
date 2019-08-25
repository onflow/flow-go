package types

import (
	"fmt"
	"strings"
	"time"

	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/ethereum/go-ethereum/rlp"
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
	hasher, _ := crypto.NewHashAlgo(crypto.SHA3_256)
	return hasher.ComputeStructHash(tx)
}

// Encode returns the encoded transaction as bytes.
func (tx *RawTransaction) Encode() []byte {
	b, _ := rlp.EncodeToBytes([]interface{}{
		tx.Script,
		tx.Nonce,
		tx.ComputeLimit,
	})
	return b
}

// SignPayer signs the transaction with the given account and keypair.
//
// The function returns a new SignedTransaction that includes the generated signature.
func (tx *RawTransaction) SignPayer(account Address, prKey crypto.PrKey) (*SignedTransaction, error) {
	// TODO: don't hard-code signature algo
	salg, err := crypto.NewSignatureAlgo(crypto.ECDSA_P256)
	if err != nil {
		return nil, err
	}

	hasher, err := crypto.NewHashAlgo(crypto.SHA3_256)
	if err != nil {
		return nil, err
	}

	sig, err := salg.SignStruct(prKey, tx, hasher)
	if err != nil {
		return nil, err
	}

	accountSig := AccountSignature{
		Account:   account,
		Signature: sig.Bytes(),
	}

	return &SignedTransaction{
		Script:         tx.Script,
		Nonce:          tx.Nonce,
		ComputeLimit:   tx.ComputeLimit,
		Timestamp:      tx.Timestamp,
		PayerSignature: accountSig,
	}, nil
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
	hasher, _ := crypto.NewHashAlgo(crypto.SHA3_256)
	return hasher.ComputeStructHash(tx)
}

// UnsignedHash computes the hash of the original unsigned transaction.
func (tx *SignedTransaction) UnsignedHash() crypto.Hash {
	return (&RawTransaction{
		tx.Script,
		tx.Nonce,
		tx.ComputeLimit,
		tx.Timestamp,
	}).Hash()
}

// Encode returns the encoded transaction as bytes.
func (tx *SignedTransaction) Encode() []byte {
	b, _ := rlp.EncodeToBytes([]interface{}{
		tx.Script,
		tx.Nonce,
		tx.ComputeLimit,
		tx.PayerSignature.Encode(),
	})
	return b
}

// InvalidTransactionError indicates that a transaction does not contain all
// required information.
type InvalidTransactionError struct {
	missingFields []string
}

func (e InvalidTransactionError) Error() string {
	return fmt.Sprintf(
		"required fields are not set: %s",
		strings.Join(e.missingFields[:], ", "),
	)
}

// Validate returns and error if the transaction does not contain all required
// fields.
func (tx *SignedTransaction) Validate() error {
	missingFields := make([]string, 0)

	if len(tx.Script) == 0 {
		missingFields = append(missingFields, "script")
	}

	if tx.ComputeLimit == 0 {
		missingFields = append(missingFields, "compute_limit")
	}

	if len(missingFields) > 0 {
		return InvalidTransactionError{missingFields}
	}

	return nil
}
