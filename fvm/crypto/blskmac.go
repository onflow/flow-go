package crypto

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
)

func NewBLSKMAC(tag string) hash.Hasher {
	return crypto.NewBLSKMAC(tag)
}
