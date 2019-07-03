package data

import (
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

// Transaction represents a normal transaction.
type Transaction struct {
	ToAddress      crypto.Address
	Script         []byte
	Nonce          uint64
	ComputeLimit   uint64
	ComputeUsed    uint64
	PayerSignature *crypto.Signature
	Status         TxStatus
}

// Hash computes the hash over the necessary transaction data.
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
