//go:build relic
// +build relic

package crypto

// #cgo CFLAGS: -g -Wall -std=c99 -I${SRCDIR}/ -I${SRCDIR}/relic/build/include
// #cgo LDFLAGS: -L${SRCDIR}/relic/build/lib -l relic_s
// #include "bls_include.h"
import "C"
import (
	"fmt"
)

// SPOCKVerify checks whether two couples of (SPoCK proof, public key) are consistent.
//
// Two (SPoCK proof, public key) couples are consistent if there exists a message such
// that each proof could be generated from the message and the private key corresponding
// to the respective public key.
//
// If the input proof slices have an invalid length or fail to deserialize into curve
// points, the function returns false without an error.
// The proofs membership checks in G1 are included in the verifcation.
//
// The function does not check the public keys membership in G2 because it is
// guaranteed by the package. However, the caller must make sure each input public key has been
// verified against a proof of possession prior to calling this function.
//
// The function returns:
//   - (false, notBLSKeyError) if at least one key is not a BLS key.
//   - (false, error) if an unexpected error occurs.
//   - (validity, nil) otherwise
func SPOCKVerify(pk1 PublicKey, proof1 Signature, pk2 PublicKey, proof2 Signature) (bool, error) {
	blsPk1, ok1 := pk1.(*pubKeyBLSBLS12381)
	blsPk2, ok2 := pk2.(*pubKeyBLSBLS12381)
	if !(ok1 && ok2) {
		return false, notBLSKeyError
	}

	if len(proof1) != signatureLengthBLSBLS12381 || len(proof2) != signatureLengthBLSBLS12381 {
		return false, nil
	}

	// if pk1 and proof1 are identities of their respective groups, any couple (pk2, proof2) would
	// verify the pairing equality which breaks the unforgeability of the SPoCK scheme. This edge case
	// is avoided by not allowing an identity pk1. Similarly, an identity pk2 is not allowed.
	if blsPk1.isIdentity || blsPk2.isIdentity {
		return false, nil
	}

	// verify the spock proof using the secret data
	verif := C.bls_spock_verify((*C.ep2_st)(&blsPk1.point),
		(*C.uchar)(&proof1[0]),
		(*C.ep2_st)(&blsPk2.point),
		(*C.uchar)(&proof2[0]))

	switch verif {
	case invalid:
		return false, nil
	case valid:
		return true, nil
	default:
		return false, fmt.Errorf("SPoCK verification failed")
	}
}
