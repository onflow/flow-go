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
	Script           []byte
	Nonce            uint64
	ComputeLimit     uint64
	ScriptSignatures []AccountSignature
	PayerSignature   AccountSignature
	Status           TransactionStatus
}

// Hash computes the hash over the necessary transaction data.
func (tx *Transaction) Hash() crypto.Hash {
	hasher, _ := crypto.NewHashAlgo(crypto.SHA3_256)
	return hasher.ComputeStructHash(tx)
}

// Encode returns the encoded transaction as bytes.
func (tx *Transaction) Encode() []byte {
	scriptSigs := make([][]byte, len(tx.ScriptSignatures))
	for i, sig := range tx.ScriptSignatures {
		scriptSigs[i] = sig.Encode()
	}

	b, _ := rlp.EncodeToBytes([]interface{}{
		tx.Script,
		tx.Nonce,
		tx.ComputeLimit,
		scriptSigs,
		tx.PayerSignature.Encode(),
	})

	return b
}

// ScriptMessage encodes the fields of the transaction used for script signatures.
func (tx *Transaction) ScriptMessage() []byte {
	b, _ := rlp.EncodeToBytes([]interface{}{
		"script",
		tx.Script,
		tx.Nonce,
		tx.ComputeLimit,
	})
	return b
}

// PayerMessage encodes the fields of the transaction used for payer signatures.
func (tx *Transaction) PayerMessage() []byte {
	scriptSigs := make([][]byte, len(tx.ScriptSignatures))
	for i, sig := range tx.ScriptSignatures {
		scriptSigs[i] = sig.Encode()
	}

	b, _ := rlp.EncodeToBytes([]interface{}{
		"payer",
		tx.Script,
		tx.Nonce,
		tx.ComputeLimit,
		scriptSigs,
	})
	return b
}

// SetPayerSignature signs the transaction with the given account and private key and sets it as the payer signature.
func (tx *Transaction) SetPayerSignature(account Address, prKey crypto.PrKey) error {
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

	sig, err := salg.SignBytes(prKey, tx.PayerMessage(), hasher)
	if err != nil {
		return err
	}

	tx.PayerSignature = AccountSignature{
		Account:   account,
		Signature: sig.Bytes(),
	}

	return nil
}

// AddScriptSignature signs the transaction with the given account and private key, then adds the signature to the list
// of script signatures.
func (tx *Transaction) AddScriptSignature(account Address, prKey crypto.PrKey) error {
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

	sig, err := salg.SignBytes(prKey, tx.ScriptMessage(), hasher)
	if err != nil {
		return err
	}

	accountSig := AccountSignature{
		Account:   account,
		Signature: sig.Bytes(),
	}

	tx.ScriptSignatures = append(tx.ScriptSignatures, accountSig)

	return nil
}
