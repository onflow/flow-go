package data

import (
	"github.com/dapperlabs/bamboo-emulator/crypto"
)

// Transaction represents a normal transaction.
type Transaction struct {
	Status         TxStatus
	ToAddress      crypto.Address
	TxData         []byte
	Nonce          uint64
	ComputeUsed    uint64
	PayerSignature []byte
}

// Hash computes the hash over the necessary Transaction data.
func (tx Transaction) Hash() crypto.Hash {
	bytes := EncodeAsBytes(
		tx.ToAddress,
		tx.TxData,
		tx.Nonce,
		tx.PayerSignature,
	)
	return crypto.NewHash(bytes)
}