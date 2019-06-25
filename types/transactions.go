package types

import (
	"github.com/dapperlabs/bamboo-emulator/crypto"
	"github.com/dapperlabs/bamboo-emulator/data"
)

// RawTransaction is an unsigned transaction.
type RawTransaction struct {
	ToAddress    crypto.Address
	Script       []byte
	Nonce        uint64
	ComputeLimit uint64
}

func (tx *RawTransaction) ToAddress() crypto.Address { return tx.ToAddress }
func (tx *RawTransaction) Script() []byte            { return tx.Script }
func (tx *RawTransaction) Nonce() uint64             { return tx.Nonce }
func (tx *RawTransaction) ComputeLimit() uint64      { return tx.ComputeLimit }

// Hash computes the hash over the necessary transaction data.
func (tx *RawTransaction) Hash() crypto.Hash {
	bytes := EncodeAsBytes(
		tx.ToAddress,
		tx.Script,
		tx.Nonce,
		tx.ComputeLimit,
	)
	return crypto.NewHash(bytes)
}

func (tx *RawTransaction) Sign(keyPair *crypto.KeyPair) *SignedTransaction {
	hash := tx.Hash()
	sig := crypto.Sign(hash, keyPair)

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
	PayerSignature []byte
	Status         data.TxStatus
}

func (tx *SignedTransaction) ToAddress() crypto.Address { return tx.ToAddress }
func (tx *SignedTransaction) Script() []byte            { return tx.Script }
func (tx *SignedTransaction) Nonce() uint64             { return tx.Nonce }
func (tx *SignedTransaction) ComputeLimit() uint64      { return tx.ComputeLimit }
func (tx *SignedTransaction) ComputeUsed() uint64       { return tx.ComputeUsed }
func (tx *SignedTransaction) PayerSignature() []byte    { return tx.PayerSignature }
func (tx *SignedTransaction) Status() data.TxStatus     { return tx.Status }

// Hash computes the hash over the necessary transaction data.
func (tx *SignedTransaction) Hash() crypto.Hash {
	bytes := EncodeAsBytes(
		tx.ToAddress,
		tx.Script,
		tx.Nonce,
		tx.ComputeLimit,
		tx.PayerSignature,
	)
	return crypto.NewHash(bytes)
}
