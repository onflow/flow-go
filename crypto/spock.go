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

// SPOCKProve generates a spock poof for data under the private key sk.
func SPOCKProve(sk PrivateKey, data []byte, kmac hash.Hasher) (Signature, error) {
	if sk.Algorithm() != BLSBLS12381 {
		return nil, errors.New("private key must be a BLS key.")
	}

	// BLS signature of data
	return sk.Sign(data, kmac)
}

// SPOCKVerifyAgainstData verifies a SPoCK proof is generated from the given data
// and the prover's public key.
func SPOCKVerifyAgainstData(pk PublicKey, proof Signature, data []byte, kmac hash.Hasher) (bool, error) {
	if pk.Algorithm() != BLSBLS12381 {
		return false, errors.New("public key must be a BLS key.")
	}
	// BLS verification of data
	return pk.Verify(proof, data, kmac)
}

// SPOCKVerify verifies a 2 SPoCK proofs are consistent against 2 public keys.
//
// 2 SPoCK proofs are consistent if there exists a message such that both proofs could
// be generated from.
func SPOCKVerify(pk1 PublicKey, proof1 Signature, pk2 PublicKey, proof2 Signature) (bool, error) {
	blsPk1, ok1 := pk1.(*PubKeyBLSBLS12381)
	blsPk2, ok2 := pk2.(*PubKeyBLSBLS12381)
	if !(ok1 && ok2) {
		return false, errors.New("public keys must be BLS keys.")
	}
	// verify the spock proof using the secret data
	return spockVerify(&blsPk1.point, proof1, &blsPk2.point, proof2), nil
}

func spockVerify(pk1 *pointG2, proof1 Signature, pk2 *pointG2, proof2 Signature) bool {
	if len(proof1) != signatureLengthBLSBLS12381 || len(proof2) != signatureLengthBLSBLS12381 {
		return false
	}
	verif := C.bls_spock_verify((*C.ep2_st)(pk1),
		(*C.uchar)(&proof1[0]),
		(*C.ep2_st)(pk2),
		(*C.uchar)(&proof2[0]))

	return (verif == valid)
}
