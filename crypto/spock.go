// +build relic

package crypto

// SPoCK design based on the BLS signature scheme
// BLS is using BLS12-381 curve

// #cgo CFLAGS: -g -Wall -std=c99 -I./ -I./relic/build/include
// #cgo LDFLAGS: -Lrelic/build/lib -l relic_s
// #include "bls_include.h"
import "C"
import (
	"errors"

	"github.com/dapperlabs/flow-go/crypto/hash"
)

//
const blsSPOCKtag = "Flow-BLS-SPOCK"

// newSPOCKkmac returns a new KMAC128 instance with the right parameters
// chosen for BLS based SPoCK.
//
// The KMAC is used for both SPoCK proofs and SPoCK verifications.
func newSPOCKkmac() hash.Hasher {
	// the error is ignored as the parameter lengths are in the correct range for kmac
	kmac, _ := hash.NewKMAC_128([]byte(blsSPOCKtag), []byte(blsKMACFunction), opSwUInputLenBLSBLS12381)
	return kmac
}

// SPOCKProve generates a spock poof for data under the private key sk.
func SPOCKProve(sk PrivateKey, data []byte) (Signature, error) {
	blsSk, ok := sk.(*PrKeyBLSBLS12381)
	if !ok {
		return nil, errors.New("private key must be a BLS key.")
	}
	kmac := newSPOCKkmac()
	// hash the input to 128 bytes
	h := kmac.ComputeHash(data)
	// BLS signature of data
	return newBLSBLS12381().blsSign(&blsSk.scalar, h), nil
}

// SPOCKVerifyAgainstData verifies a SPoCK proof is generated from the given data
// and the prover's public key.
func SPOCKVerifyAgainstData(s Signature, pk PublicKey, data []byte) (bool, error) {
	blsPk, ok := pk.(*PubKeyBLSBLS12381)
	if !ok {
		return false, errors.New("public key must be a BLS key.")
	}
	kmac := newSPOCKkmac()
	// hash the input to 128 bytes
	h := kmac.ComputeHash(data)
	// verify the spock proof using the secret data
	return newBLSBLS12381().blsVerify(&blsPk.point, s, h), nil
}
