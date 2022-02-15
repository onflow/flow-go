// +build relic

package crypto

import (
	"fmt"

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
		// at this stage, the runtime public key is valid and there are no possible user value errors
		panic(fmt.Errorf("verify PoP failed: runtime public key should be valid %x", pk.PublicKey))
	}

	valid, err := crypto.BLSVerifyPOP(key, s)
	if err != nil {
		// no user errors possible at this stage
		panic(fmt.Errorf("verify PoP failed with unexpected error %w", err))
	}
	return valid, nil
}

// AggregateSignatures aggregate multiple signatures into one; currently only works for BLS
func AggregateSignatures(sigs [][]byte) (crypto.Signature, error) {
	s := make([]crypto.Signature, 0, len(sigs))
	for _, sig := range sigs {
		s = append(s, sig)
	}

	aggregatedSignature, err := crypto.AggregateBLSSignatures(s)
	if err != nil {
		// check for a user error
		if crypto.IsInvalidInputsError(err) {
			return nil, err
		}
		panic(fmt.Errorf("aggregate BLS signatures failed with unexpected error %w", err))
	}
	return aggregatedSignature, nil
}

// AggregatePublicKeys aggregate multiple public keys into one; currently only works for BLS
func AggregatePublicKeys(keys []*runtime.PublicKey) (*runtime.PublicKey, error) {
	pks := make([]crypto.PublicKey, 0, len(keys))
	for _, key := range keys {
		// TODO: avoid validating the public keys again since Cadence makes sure runtime keys have been validated.
		// This requires exporting an unsafe function in the crypto package.
		pk, err := crypto.DecodePublicKey(crypto.BLSBLS12381, key.PublicKey)
		if err != nil {
			// at this stage, the runtime public key is valid and there are no possible user value errors
			panic(fmt.Errorf("aggregate BLS public keys failed: runtime public key should be valid %x", key.PublicKey))
		}
		pks = append(pks, pk)
	}

	pk, err := crypto.AggregateBLSPublicKeys(pks)
	if err != nil {
		// check for a user error
		if crypto.IsInvalidInputsError(err) {
			return nil, err
		}
		panic(fmt.Errorf("aggregate BLS public keys failed with unexpected error %w", err))
	}

	return &runtime.PublicKey{
		PublicKey: pk.Encode(),
		SignAlgo:  CryptoToRuntimeSigningAlgorithm(crypto.BLSBLS12381),
	}, nil
}
