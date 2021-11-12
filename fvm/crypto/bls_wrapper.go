// +build !relic

package crypto

import (
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
)

func NewBLSKMAC(_ string) hash.Hasher {
	panic("BLSKMAC not supported when flow-go is built without relic")
}

// VerifyPOP verifies a proof of possession (PoP) for the receiver public key; currently only works for BLS
func VerifyPOP(pk *runtime.PublicKey, s crypto.Signature) (bool, error) {
	panic("VerifyPOP not supported with non-relic build")
}

// AggregateSignatures aggregate multiple signatures into one; currently only works for BLS
func AggregateSignatures(sigs [][]byte) (crypto.Signature, error) {
	panic("AggregateSignatures not supported with non-relic build")
}

// AggregatePublicKeys aggregate multiple public keys into one; currently only works for BLS
func AggregatePublicKeys(keys []*runtime.PublicKey) (*runtime.PublicKey, error) {
	panic("AggregatePublicKeys not supported with non-relic build")
}
