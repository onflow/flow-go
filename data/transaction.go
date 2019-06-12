package data

import (
	"github.com/dapperlabs/bamboo-emulator/crypto"
)

// Transaction represents a normal transaction.
type Transaction struct {
	ToAddress      crypto.Address
	Script         []byte
	Nonce          uint64
	ComputeLimit   uint64
	ComputeUsed    uint64
	PayerSignature []byte
	Status         TxStatus
}

// Hash computes the hash over the necessary Transaction data.
func (tx Transaction) Hash() crypto.Hash {
	bytes := EncodeAsBytes(
		tx.ToAddress,
		tx.Script,
		tx.Nonce,
		tx.ComputeLimit,
		tx.PayerSignature,
	)
	return crypto.NewHash(bytes)
}
