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
// The KMAC is used for both SPoCK proofs and SPoCK verifications against data.
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
func SPOCKVerifyAgainstData(pk PublicKey, s Signature, data []byte) (bool, error) {
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

// SPOCKVerify verifies a 2 SPoCK proofs are consistent against 2 public keys.
//
// 2 SPoCK proofs are consistent if there exists a message such that both proofs could
// be generated from.
func SPOCKVerify(pk1 PublicKey, s1 Signature, pk2 PublicKey, s2 Signature) (bool, error) {
	blsPk1, ok1 := pk1.(*PubKeyBLSBLS12381)
	blsPk2, ok2 := pk2.(*PubKeyBLSBLS12381)
	if !(ok1 && ok2) {
		return false, errors.New("public keys must be BLS keys.")
	}
	// verify the spock proof using the secret data
	return spockVerify(&blsPk1.point, s1, &blsPk2.point, s2), nil
}

func spockVerify(pk1 *pointG2, s1 Signature, pk2 *pointG2, s2 Signature) bool {
	if len(s1) != signatureLengthBLSBLS12381 || len(s2) != signatureLengthBLSBLS12381 {
		return false
	}
	verif := C.bls_spock_verify((*C.ep2_st)(pk1),
		(*C.uchar)(&s1[0]),
		(*C.ep2_st)(pk2),
		(*C.uchar)(&s2[0]))

	return (verif == valid)
}
