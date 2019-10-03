package types

import (
	"github.com/dapperlabs/flow-go/pkg/crypto"
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
	hasher, _ := crypto.NewHashAlgo(crypto.SHA3_256)
	return hasher.ComputeBytesHash(tx.CanonicalEncoding())
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
func (tx *Transaction) AddSignature(account Address, prKey crypto.PrKey) error {
	// TODO: replace hard-coded signature algorithm
	salg, err := crypto.NewSignatureAlgo(crypto.ECDSA_P256)
	if err != nil {
		return err
	}

	// TODO: replace hard-coded hashing algorithm
	hasher, err := crypto.NewHashAlgo(crypto.SHA3_256)
	if err != nil {
		return err
	}

	sig, err := salg.SignBytes(prKey, tx.CanonicalEncoding(), hasher)
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
