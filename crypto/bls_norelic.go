//go:build !relic
// +build !relic

package crypto

// Stubs of bls_relic.go for non relic build of flow-go.

import (
	"github.com/onflow/flow-go/crypto/hash"
)

func NewBLSKMAC(_ string) hash.Hasher {
	panic("BLSKMAC not supported when flow-go is built without relic")
}
