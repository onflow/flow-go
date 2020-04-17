package utils

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/crypto/hash"
)

// NewExecutionReceiptHasher generates and returns a hasher for signing
// and verification of execution receipts
func NewExecutionReceiptHasher() hash.Hasher {
	h := crypto.NewBLS_KMAC("execution receipt")
	return h
}
