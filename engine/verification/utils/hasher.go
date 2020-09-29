package utils

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/encoding"
)

// NewResultApprovalHasher generates and returns a hasher for signing
// and verification of result approvals
func NewResultApprovalHasher() hash.Hasher {
	h := crypto.NewBLSKMAC(encoding.ResultApprovalTag)
	return h
}
