// +build !relic

package crypto

import "github.com/onflow/flow-go/crypto/hash"

func NewBLSKMAC(_ string) hash.Hasher {
	panic("BLSKMAC not supported when flow-go is built without relic")
}
