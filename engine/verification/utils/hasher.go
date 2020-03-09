package utils

import (
	"encoding/binary"
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/crypto/random"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/flow"
)

// NewResultApprovalHasher generates and returns a hasher for signing
// and verification of result approvals
func NewResultApprovalHasher() crypto.Hasher {
	h := crypto.NewBLS_KMAC("result approval")
	return h
}

// NewChunkAssignmentRNG generates and returns a hasher for chunk
// assignment
func NewChunkAssignmentRNG(res *flow.ExecutionResult) (random.RandomGenerator, error) {
	h, err := crypto.NewHasher(crypto.SHA3_384)
	if err != nil {
		return nil, fmt.Errorf("could not generate hasher: %w", err)
	}

	// encodes result approval body to byte slice
	b, err := encoding.DefaultEncoder.Encode(res.ExecutionResultBody)
	if err != nil {
		return nil, fmt.Errorf("could not encode execution result body: %w", err)
	}

	// takes hash of result approval body
	hash := h.ComputeHash(b)

	// converts hash to slice of uint64
	// stratifies every 8 consecutive bytes into a uint64
	l := len(hash) / 8
	uhash := make([]uint64, l)
	for i := 0; i < len(hash); i += 8 {
		uhash[i/8] = binary.LittleEndian.Uint64(hash[i : i+8])
	}

	// creates a random generator
	rng, err := random.NewRand(uhash)
	if err != nil {
		return nil, fmt.Errorf("could not generate random generator: %w", err)
	}

	return rng, nil
}
