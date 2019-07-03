package types

import (
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/internal/emulator/data"
)

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
	Status         data.TxStatus
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
