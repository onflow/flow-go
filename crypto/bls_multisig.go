// +build relic

package crypto

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto/hash"
)

// BLS multi-signature using BLS12-381 curve
// ([zcash]https://github.com/zkcrypto/pairing/blob/master/src/bls12_381/README.md#bls12-381)
// Pairing, ellipic curve and modular arithmetic is using Relic library.
// This implementation does not include any security against side-channel attacks.

// existing features:
//  - the same BLS set-up in bls.go
//  - Use the proof of possession (PoP) to prevent the Rogue public-key attack.
//  - Non-interactive aggregation of multiple signatures into one.
//  - Non-interactive aggregation of multiple private keys into one.
//  - Non-interactive aggregation of multiple public keys into one.
//  - Non-interactive subtraction of multiple public keys from an aggregated public key.
//  - Multi-signature verification of an aggregated signature of a single message under multiple
//  public keys.

// #cgo CFLAGS: -g -Wall -std=c99 -I./ -I./relic/build/include
// #cgo LDFLAGS: -Lrelic/build/lib -l relic_s
// #include "bls_include.h"
import "C"

// AggregateSignatures aggregate multiple BLS signatures into one.
//
// Signatures could be generated from the same or distinct messages, they
// could also be the aggregation of other signatures.
// The order of the signatures in the slice does not matter since the aggregation
// is commutative.
// No subgroup membership check is performed on the input signatures.
func AggregateSignatures(sigs []Signature) (Signature, error) {
	_ = newBLSBLS12381()
	// flatten the shares (required by the C layer)
	flatSigs := make([]byte, 0, signatureLengthBLSBLS12381*len(sigs))
	for _, sig := range sigs {
		flatSigs = append(flatSigs, sig...)
	}
	aggregatedSig := make([]byte, signatureLengthBLSBLS12381)

	var sigsPointer *C.uchar
	if len(sigs) != 0 {
		sigsPointer = (*C.uchar)(&flatSigs[0])
	} else {
		sigsPointer = (*C.uchar)(nil)
	}

	// add the points in the C layeer
	if C.ep_sum_vector(
		(*C.uchar)(&aggregatedSig[0]),
		sigsPointer,
		(C.int)(len(sigs)),
	) != valid {
		return nil, fmt.Errorf("decoding BLS signatures has failed")
	}
	return aggregatedSig, nil
}

// AggregatePrivateKeys aggregate multiple BLS private keys into one.
//
// The order of the keys in the slice does not matter since the aggregation
// is commutative. The slice can be empty.
// No check is performed on the input private keys.
func AggregatePrivateKeys(keys []PrivateKey) (PrivateKey, error) {
	_ = newBLSBLS12381()
	scalars := make([]scalar, 0, len(keys))
	for _, sk := range keys {
		if sk.Algorithm() != BLSBLS12381 {
			return nil, fmt.Errorf("all keys must be BLS keys")
		}
		// assertion is guaranteed to be correct after the algorithm check
		skBls, _ := sk.(*PrKeyBLSBLS12381)
		scalars = append(scalars, skBls.scalar)
	}

	var sum scalar
	var scalarPointer *C.bn_st
	if len(keys) != 0 {
		scalarPointer = (*C.bn_st)(&scalars[0])
	} else {
		scalarPointer = (*C.bn_st)(nil)
	}
	C.bn_sum_vector((*C.bn_st)(&sum), scalarPointer,
		(C.int)(len(scalars)))
	return &PrKeyBLSBLS12381{
		pk:     nil,
		scalar: sum,
	}, nil
}

// AggregatePublicKeys aggregate multiple BLS public keys into one.
//
// The order of the keys in the slice does not matter since the aggregation
// is commutative. The slice can be empty.
// No check is performed on the input public keys.
func AggregatePublicKeys(keys []PublicKey) (PublicKey, error) {
	_ = newBLSBLS12381()
	points := make([]pointG2, 0, len(keys))
	for _, pk := range keys {
		if pk.Algorithm() != BLSBLS12381 {
			return nil, fmt.Errorf("all keys must be BLS keys")
		}
		// assertion is guaranteed to be correct after the algorithm check
		pkBLS, _ := pk.(*PubKeyBLSBLS12381)
		points = append(points, pkBLS.point)
	}

	var sum pointG2
	var pointsPointer *C.ep2_st
	if len(keys) != 0 {
		pointsPointer = (*C.ep2_st)(&points[0])
	} else {
		pointsPointer = (*C.ep2_st)(nil)
	}
	C.ep2_sum_vector((*C.ep2_st)(&sum), pointsPointer,
		(C.int)(len(points)))
	return &PubKeyBLSBLS12381{
		point: sum,
	}, nil
}

// RemovePublicKeys removes multiple BLS public keys from a given (aggregated) public key.
//
// The common use case assumes the aggregated public key was initially formed using
// the keys to be removed (directly or using other aggregated forms). However the function
// can still be called in different use cases.
// The order of the keys to be removed in the slice does not matter since the removal
// is commutative. The slice of keys to be removed can be empty.
// No check is performed on the input public keys.
func RemovePublicKeys(aggKey PublicKey, keysToRemove []PublicKey) (PublicKey, error) {
	_ = newBLSBLS12381()
	if aggKey.Algorithm() != BLSBLS12381 {
		return nil, fmt.Errorf("all keys must be BLS keys")
	}
	// assertion is guaranteed to be correct after the algorithm check
	aggPKBLS, _ := aggKey.(*PubKeyBLSBLS12381)

	pointsToSubtract := make([]pointG2, 0, len(keysToRemove))
	for _, pk := range keysToRemove {
		if pk.Algorithm() != BLSBLS12381 {
			return nil, fmt.Errorf("all keys must be BLS keys")
		}
		// assertion is guaranteed to be correct after the algorithm check
		pkBLS, _ := pk.(*PubKeyBLSBLS12381)
		pointsToSubtract = append(pointsToSubtract, pkBLS.point)
	}

	var pointsPointer *C.ep2_st
	if len(pointsToSubtract) != 0 {
		pointsPointer = (*C.ep2_st)(&pointsToSubtract[0])
	} else {
		pointsPointer = (*C.ep2_st)(nil)
	}

	var resultKey pointG2
	C.ep2_subtract_vector((*C.ep2_st)(&resultKey), (*C.ep2_st)(&aggPKBLS.point),
		pointsPointer, (C.int)(len(pointsToSubtract)))

	return &PubKeyBLSBLS12381{
		point: resultKey,
	}, nil
}

// VerifySignatureOneMessage is a multi-signature verification that verifies a
// BLS signature of a single message against multiple BLS public keys.
//
// The input signature could be generated by aggregating multiple signatures of the
// message under multiple private keys. The public keys corresponding to the signing
// private keys are passed as input to this function.
// The order of the public keys in the slice does not matter. An error is returned if
// the slice is empty.
// Membership check is performed on the input signature.
func VerifySignatureOneMessage(pks []PublicKey, s Signature,
	message []byte, kmac hash.Hasher) (bool, error) {
	// check the public key list is non empy
	if len(pks) == 0 {
		return false, fmt.Errorf("key list is empty")
	}
	aggPk, err := AggregatePublicKeys(pks)
	if err != nil {
		return false, fmt.Errorf("aggregating public keys for verification failed: %w", err)
	}
	return aggPk.Verify(s, message, kmac)
}
