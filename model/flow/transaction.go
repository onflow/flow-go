// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"github.com/dapperlabs/flow-go/crypto"
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

// TransactionField represents a required transaction field.
type TransactionField int

const (
	TransactionFieldScript TransactionField = iota
	TransactionFieldRefBlockHash
	TransactionFieldNonce
	TransactionFieldComputeLimit
	TransactionFieldPayerAccount
)

// String returns the string representation of a transaction field.
func (f TransactionField) String() string {
	return [...]string{"Script", "ReferenceBlockHash", "Nonce", "ComputeLimit", "PayerAccount"}[f]
}

// Transaction is a transaction that contains a script and optional signatures.
type Transaction struct {
	Hash               crypto.Hash
	Script             []byte
	ReferenceBlockHash []byte
	Nonce              uint64
	ComputeLimit       uint64
	PayerAccount       Address
	ScriptAccounts     []Address
	Signatures         []AccountSignature
	Status             TransactionStatus
}

// AddSignature signs the transaction with the given account and private key, then adds the signature to the list
// of signatures.
func (tx *Transaction) AddSignature(account Address, sig crypto.Signature) {
	accountSig := AccountSignature{
		Account:   account,
		Signature: sig.Bytes(),
	}

	tx.Signatures = append(tx.Signatures, accountSig)
}

// MissingFields checks if a transaction is missing any required fields and returns those that are missing.
func (tx *Transaction) MissingFields() []string {
	// Required fields are Script, ReferenceBlockHash, Nonce, ComputeLimit, PayerAccount
	missingFields := make([]string, 0)

	if len(tx.Script) == 0 {
		missingFields = append(missingFields, TransactionFieldScript.String())
	}

	// TODO: need to refactor tests to include ReferenceBlockHash field (i.e. b.GetLatestBlock().Hash() should do)
	// if len(tx.ReferenceBlockHash) == 0 {
	// 	missingFields = append(missingFields, TransactionFieldRefBlockHash.String())
	// }

	if tx.Nonce == 0 {
		missingFields = append(missingFields, TransactionFieldNonce.String())
	}

	if tx.ComputeLimit == 0 {
		missingFields = append(missingFields, TransactionFieldComputeLimit.String())
	}

	if tx.PayerAccount == ZeroAddress {
		missingFields = append(missingFields, TransactionFieldPayerAccount.String())
	}

	return missingFields
}
