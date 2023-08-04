package utils

import (
	"fmt"

	"github.com/onflow/crypto/hash"
	"github.com/onflow/flow-go/module/signature"
)

// NewExecutionReceiptHasher generates and returns a hasher for signing
// and verification of execution receipts
func NewExecutionReceiptHasher() hash.Hasher {
	h := signature.NewBLSHasher(signature.ExecutionReceiptTag)
	return h
}

// NewSPOCKHasher generates and returns a hasher for signing
// and verification of SPoCKs
func NewSPOCKHasher() hash.Hasher {
	h := signature.NewBLSHasher(signature.SPOCKTag)
	return h
}

// NewHasher returns one of the hashers supported by Flow transactions.
func NewHasher(algo hash.HashingAlgorithm) (hash.Hasher, error) {
	switch algo {
	case hash.SHA2_256:
		return hash.NewSHA2_256(), nil
	case hash.SHA3_256:
		return hash.NewSHA3_256(), nil
	default:
		return nil, fmt.Errorf("not supported hashing algorithms: %d", algo)
	}
}
