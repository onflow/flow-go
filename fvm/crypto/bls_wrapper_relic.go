// +build relic

package crypto

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
)

func NewBLSKMAC(tag string) hash.Hasher {
	return crypto.NewBLSKMAC(tag)
}

// VerifyPOP verifies a proof of possession (PoP) for the receiver public key; currently only works for BLS
func VerifyPOP(pk crypto.PublicKey, s crypto.Signature) (bool, error) {
	return BLSVerifyPOP(pk, s)
}

// AggregateSignatures aggregate multiple signatures into one; currently only works for BLS
func AggregateSignatures(sigs [][]byte) (crypto.Signature, error) {
	s := make([]Signature, 0, len(sigs))
	for _, sig := range sigs {
		s = append(s, sig)
	}
	return AggregateBLSSignatures(s)
}

// AggregatePublicKeys aggregate multiple public keys into one; currently only works for BLS
func AggregatePublicKeys(keys []crypto.PublicKey) (crypto.PublicKey, error) {
	return AggregateBLSPublicKeys(keys)
}
