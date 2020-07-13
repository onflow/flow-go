// +build relic

package crypto

// SPoCK design based on the BLS signature scheme
// BLS is using BLS12-381 curve

// #cgo CFLAGS: -g -Wall -std=c99 -I./ -I./relic/build/include
// #cgo LDFLAGS: -Lrelic/build/lib -l relic_s
// #include "bls_include.h"
import "C"
import (
	"bytes"
	"errors"

	"github.com/dapperlabs/flow-go/crypto/hash"
)

// This is only a temporary implementation of the SPoCK API. The purpose
// is only to facilitate the SPoCK integration in Flow. The implementation
// passes the happy and unhappy path tests but does not satisfy any security
// property.

// SPOCKProve generates a spock proof for data under the private key sk.
func SPOCKProve(sk PrivateKey, data []byte, kmac hash.Hasher) (Signature, error) {
	_, ok := sk.(*PrKeyBLSBLS12381)
	if !ok {
		return nil, errors.New("private key must be a BLS key.")
	}

	s := make([]byte, SignatureLenBLSBLS12381)
	copy(s[0:], sk.PublicKey().Encode()[:10])
	copy(s[10:], data)

	return Signature(s), nil
}

// SPOCKVerifyAgainstData verifies a SPoCK proof is generated from the given data
// and the prover's public key.
func SPOCKVerifyAgainstData(pk PublicKey, proof Signature, data []byte, kmac hash.Hasher) (bool, error) {
	_, ok := pk.(*PubKeyBLSBLS12381)
	if !ok {
		return false, errors.New("public key must be a BLS key.")
	}
	var minLen int
	if len(data) < SignatureLenBLSBLS12381-10 {
		minLen = len(data)
	} else {
		minLen = SignatureLenBLSBLS12381 - 10
	}
	return bytes.Equal(pk.Encode()[:10], proof[:10]) &&
		bytes.Equal(data[:minLen], proof[10:10+minLen]), nil
}

// SPOCKVerify verifies a 2 SPoCK proofs are consistent against 2 public keys.
//
// 2 SPoCK proofs are consistent if there exists a message such that both proofs could
// be generated from.
func SPOCKVerify(pk1 PublicKey, proof1 Signature, pk2 PublicKey, proof2 Signature) (bool, error) {
	_, ok1 := pk1.(*PubKeyBLSBLS12381)
	_, ok2 := pk2.(*PubKeyBLSBLS12381)
	if !(ok1 && ok2) {
		return false, errors.New("public keys must be BLS keys.")
	}
	return bytes.Equal(pk1.Encode()[:10], proof1[:10]) &&
		bytes.Equal(pk2.Encode()[:10], proof2[:10]) &&
		bytes.Equal(proof1[10:], proof2[10:]), nil
}
