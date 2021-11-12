// +build relic

package crypto

import (
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
)

func NewBLSKMAC(tag string) hash.Hasher {
	return crypto.NewBLSKMAC(tag)
}

// VerifyPOP verifies a proof of possession (PoP) for the receiver public key; currently only works for BLS
func VerifyPOP(pk *runtime.PublicKey, s crypto.Signature) (bool, error) {
	key, err := crypto.DecodePublicKey(crypto.BLSBLS12381, pk.PublicKey)
	if err != nil {
		return false, err
	}
	return crypto.BLSVerifyPOP(key, s)
}

// AggregateSignatures aggregate multiple signatures into one; currently only works for BLS
func AggregateSignatures(sigs [][]byte) (crypto.Signature, error) {
	s := make([]crypto.Signature, 0, len(sigs))
	for _, sig := range sigs {
		s = append(s, sig)
	}
	return crypto.AggregateBLSSignatures(s)
}

// AggregatePublicKeys aggregate multiple public keys into one; currently only works for BLS
func AggregatePublicKeys(keys []*runtime.PublicKey) (*runtime.PublicKey, error) {
	pks := make([]crypto.PublicKey, 0, len(keys))
	for _, key := range keys {
		pk, err := crypto.DecodePublicKey(crypto.BLSBLS12381, key.PublicKey)
		if err != nil {
			return nil, err
		}
		pks = append(pks, pk)
	}
	pk, err := crypto.AggregateBLSPublicKeys(pks)
	if err != nil {
		return nil, err
	}

	return &runtime.PublicKey{
		PublicKey: pk.Encode(),
		SignAlgo:  CryptoToRuntimeSigningAlgorithm(crypto.BLSBLS12381),
	}, nil
}
