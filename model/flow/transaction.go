// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"github.com/dapperlabs/flow-go/crypto"
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
	Script             []byte
	ReferenceBlockHash []byte
	Nonce              uint64
	ComputeLimit       uint64
	PayerAccount       Address
	ScriptAccounts     []Address
	Signatures         []AccountSignature
	Status             TransactionStatus
}

// Hash computes the hash over the necessary transaction data.
func (tx *Transaction) Hash() crypto.Hash {
	hasher, _ := crypto.NewHasher(crypto.SHA3_256)
	return hasher.ComputeHash(tx.CanonicalEncoding())
}

// CanonicalEncoding returns the encoded canonical transaction as bytes.
func (tx *Transaction) CanonicalEncoding() []byte {
	b, _ := rlp.EncodeToBytes([]interface{}{
		tx.Script,
		tx.ReferenceBlockHash,
		tx.Nonce,
		tx.ComputeLimit,
		tx.PayerAccount,
		tx.ScriptAccounts,
	})

	return b
}

// FullEncoding returns the encoded full transaction (including signatures) as bytes.
func (tx *Transaction) FullEncoding() []byte {
	sigs := make([][]byte, len(tx.ScriptAccounts))
	for i, sig := range tx.Signatures {
		sigs[i] = sig.Encode()
	}

	b, _ := rlp.EncodeToBytes([]interface{}{
		tx.Script,
		tx.ReferenceBlockHash,
		tx.Nonce,
		tx.ComputeLimit,
		tx.PayerAccount,
		tx.ScriptAccounts,
		sigs,
	})

	return b
}

// AddSignature signs the transaction with the given account and private key, then adds the signature to the list
// of signatures.
func (tx *Transaction) AddSignature(account Address, prKey crypto.PrivateKey) error {
	// TODO: replace hard-coded hashing algorithm
	hasher, err := crypto.NewHasher(crypto.SHA3_256)
	if err != nil {
		return err
	}

	sig, err := prKey.Sign(tx.CanonicalEncoding(), hasher)
	if err != nil {
		return err
	}

	accountSig := AccountSignature{
		Account:   account,
		Signature: sig.Bytes(),
	}

	tx.Signatures = append(tx.Signatures, accountSig)

	return nil
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
