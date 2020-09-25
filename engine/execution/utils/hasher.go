package utils

import (
	"fmt"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/encoding"
)

// NewExecutionReceiptHasher generates and returns a hasher for signing
// and verification of execution receipts
func NewExecutionReceiptHasher() hash.Hasher {
	h := crypto.NewBLSKMAC(encoding.ExecutionReceiptTag)
	return h
}

// NewSPOCKHasher generates and returns a hasher for signing
// and verification of SPoCKs
func NewSPOCKHasher() hash.Hasher {
	h := crypto.NewBLSKMAC(encoding.SPOCKTag)
	return h
}

// NewHasher returns one of the hashers supported by Flow transactions.
func NewHasher(algo hash.HashingAlgorithm) (hash.Hasher, error) {
	switch algo {
	case hash.SHA2_256:
		return hash.NewSHA2_256(), nil
	case hash.SHA2_384:
		return hash.NewSHA2_384(), nil
	case hash.SHA3_256:
		return hash.NewSHA3_256(), nil
	case hash.SHA3_384:
		return hash.NewSHA3_384(), nil
	}
	return nil, fmt.Errorf("not supported hashing algorithms: %d", algo)
}
