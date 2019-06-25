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

func (tx *RawTransaction) GetToAddress() crypto.Address { return tx.ToAddress }
func (tx *RawTransaction) GetScript() []byte            { return tx.Script }
func (tx *RawTransaction) GetNonce() uint64             { return tx.Nonce }
func (tx *RawTransaction) GetComputeLimit() uint64      { return tx.ComputeLimit }

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

func (tx *SignedTransaction) GetToAddress() crypto.Address         { return tx.ToAddress }
func (tx *SignedTransaction) GetScript() []byte                    { return tx.Script }
func (tx *SignedTransaction) GetNonce() uint64                     { return tx.Nonce }
func (tx *SignedTransaction) GetComputeLimit() uint64              { return tx.ComputeLimit }
func (tx *SignedTransaction) GetComputeUsed() uint64               { return tx.ComputeUsed }
func (tx *SignedTransaction) GetPayerSignature() *crypto.Signature { return tx.PayerSignature }
func (tx *SignedTransaction) GetStatus() data.TxStatus             { return tx.Status }

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
