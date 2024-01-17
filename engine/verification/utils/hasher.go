package utils

import (
	"github.com/onflow/crypto/hash"

	"github.com/onflow/flow-go/module/signature"
)

// NewResultApprovalHasher generates and returns a hasher for signing
// and verification of result approvals
func NewResultApprovalHasher() hash.Hasher {
	h := signature.NewBLSHasher(signature.ResultApprovalTag)
	return h
}
