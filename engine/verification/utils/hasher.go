package utils

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/crypto/random"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/flow"
)

// NewResultApprovalHasher generates and returns a hasher for signing
// and verification of result approvals
func NewResultApprovalHasher() hash.Hasher {
	h := crypto.NewBLSKMAC(encoding.ResultApprovalTag)
	return h
}

// NewChunkAssignmentRNG generates and returns a hasher for chunk
// assignment
func NewChunkAssignmentRNG(res *flow.ExecutionResult) (random.Rand, error) {
	h := hash.NewSHA3_384()

	// encodes result approval body to byte slice
	b, err := encoding.DefaultEncoder.Encode(res.ExecutionResultBody)
	if err != nil {
		return nil, fmt.Errorf("could not encode execution result body: %w", err)
	}

	// takes hash of result approval body
	hash := h.ComputeHash(b)

	// creates a random generator
	rng, err := random.NewRand(hash)
	if err != nil {
		return nil, fmt.Errorf("could not generate random generator: %w", err)
	}

	return rng, nil
}
