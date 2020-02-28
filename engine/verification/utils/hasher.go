package utils

import (
	"github.com/dapperlabs/flow-go/crypto"
)

// NewResultApprovalHasher generates and returns a hasher for signing
// and verification of result approvals
func NewResultApprovalHasher() crypto.Hasher {
	h := crypto.NewBLS_KMAC("result approval")
	return h
}
