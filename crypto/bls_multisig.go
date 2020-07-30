// +build relic

package crypto

import (
	"fmt"
)

// BLS multi signature using BLS12-381 curve
// ([zcash]https://github.com/zkcrypto/pairing/blob/master/src/bls12_381/README.md#bls12-381)
// Pairing, ellipic curve and modular arithmetic is using Relic library.
// This implementation does not include any security against side-channel attacks.

// existing features:
//  - the same BLS set-up in bls.go
//  - Use the proof of possession (PoP) to prevent the Rogue public-key attack.
//  - Non-interactive aggregation of multiple signatures into one.
//  - Non-interactive aggregation of multiple private keys into one.
//  - Non-interactive aggregation of multiple public keys into one.

// #cgo CFLAGS: -g -Wall -std=c99 -I./ -I./relic/build/include
// #cgo LDFLAGS: -Lrelic/build/lib -l relic_s
// #include "bls_include.h"
import "C"

// AggregateSignatures aggregate multiple BLS signatures into one.
//
// Signatures could be generated from the same or distinct messages, they
// could also be the aggregation of other signatures.
// The order of the signatures in the slice does not matter since the aggregation
// is commutative. An error is returned if the slice is empty.
// No subgroup membership check is performed on the input signatures.
func AggregateSignatures(sigs []Signature) (Signature, error) {
	_ = newBLSBLS12381()
	// TODO: check if it's fine to return identity g_1
	if len(sigs) == 0 {
		return nil, fmt.Errorf("signature list is empty")
	}
	// flatten the shares (required by the C layer)
	flatSigs := make([]byte, 0, signatureLengthBLSBLS12381*len(sigs))
	for _, sig := range sigs {
		flatSigs = append(flatSigs, sig...)
	}
	aggregatedSig := make([]byte, signatureLengthBLSBLS12381)

	// add the points in the C layeer
	if C.ep_sum_vector(
		(*C.uchar)(&aggregatedSig[0]),
		(*C.uchar)(&flatSigs[0]),
		(C.int)(len(sigs)),
	) != valid {
		return nil, fmt.Errorf("decoding signatures has failed")
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
