package utils

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/model/encoding"
)

// NewExecutionReceiptHasher generates and returns a hasher for signing
// and verification of execution receipts
func NewExecutionReceiptHasher() hash.Hasher {
	h := crypto.NewBLSKMAC(encoding.ExecutionReceiptTag)
	return h
}

// NewSPoCKHasher generates and returns a hasher for signing
// and verification of SPoCKs
func NewSPoCKHasher() hash.Hasher {
	h := crypto.NewBLSKMAC(encoding.SPoCKTag)
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
