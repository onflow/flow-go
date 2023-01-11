//go:build relic
// +build relic

package crypto

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/crypto/hash"
)

// BLS multi-signature using BLS12-381 curve
// ([zcash]https://github.com/zkcrypto/pairing/blob/master/src/bls12_381/README.md#bls12-381)
// Pairing, ellipic curve and modular arithmetic is using Relic library.
// This implementation does not include any security against side-channel attacks.

// existing features:
//  - the same BLS set-up in bls.go
//  - Use the proof of possession scheme (PoP) to prevent against rogue public-key attack.
//  - Non-interactive aggregation of private keys, public keys and signatures.
//  - Non-interactive subtraction of multiple public keys from an (aggregated) public key.
//  - Multi-signature verification of an aggregated signature of a single message
//  under multiple public keys.
//  - Multi-signature verification of an aggregated signature of multiple messages under
//  multiple public keys.
//  - batch verification of multiple signatures of a single message under multiple
//  public keys: use a binary tree of aggregations to find the invalid signatures.

// #cgo CFLAGS: -g -Wall -std=c99
// #cgo LDFLAGS: -L${SRCDIR}/relic/build/lib -l relic_s
// #include "bls_include.h"
import "C"

// the PoP hasher, used to generate and verify PoPs
// The key is based on blsPOPCipherSuite which guarantees
// that hash_to_field of PoP is orthogonal to all hash_to_field functions
// used for signatures.
var popKMAC = internalExpandMsgXOFKMAC128(blsPOPCipherSuite)

// BLSGeneratePOP returns a proof of possession (PoP) for the receiver private key.
//
// The KMAC hasher used in the function is guaranteed to be orthogonal to all hashers used
// for signatures or SPoCK proofs on this package. This means a specific domain tag is used
// to generate PoP and is not used by any other application.
//
// The function returns:
//   - (nil, notBLSKeyError) if the input key is not of type BLS BLS12-381
//   - (pop, nil) otherwise
func BLSGeneratePOP(sk PrivateKey) (Signature, error) {
	_, ok := sk.(*prKeyBLSBLS12381)
	if !ok {
		return nil, notBLSKeyError
	}
	// sign the public key
	return sk.Sign(sk.PublicKey().Encode(), popKMAC)
}

// BLSVerifyPOP verifies a proof of possession (PoP) for the receiver public key.
//
// The function internally uses the same KMAC hasher used to generate the PoP.
// The hasher is guaranteed to be orthogonal to any hasher used to generate signature
// or SPoCK proofs on this package.
// Note that verifying a PoP against an idenity public key fails.
//
// The function returns:
//   - (false, notBLSKeyError) if the input key is not of type BLS BLS12-381
//   - (validity, nil) otherwise
func BLSVerifyPOP(pk PublicKey, s Signature) (bool, error) {
	_, ok := pk.(*pubKeyBLSBLS12381)
	if !ok {
		return false, notBLSKeyError
	}
	// verify the signature against the public key
	return pk.Verify(s, pk.Encode(), popKMAC)
}

// AggregateBLSSignatures aggregates multiple BLS signatures into one.
//
// Signatures could be generated from the same or distinct messages, they
// could also be the aggregation of other signatures.
// The order of the signatures in the slice does not matter since the aggregation
// is commutative. The slice should not be empty.
// No G1 membership check is performed on the input signatures.
//
// The function returns:
//   - (nil, blsAggregateEmptyListError) if no signatures are provided (input slice is empty)
//   - (nil, invalidSignatureError) if a deserialization of at least one signature fails (input is an invalid serialization of a
//     compressed E1 element following [zcash]
//     https://www.ietf.org/archive/id/draft-irtf-cfrg-pairing-friendly-curves-08.html#name-zcash-serialization-format-).
//     G1 membership is not checked.
//   - (nil, error) if an unexpected error occurs
//   - (aggregated_signature, nil) otherwise
func AggregateBLSSignatures(sigs []Signature) (Signature, error) {
	// set BLS context
	blsInstance.reInit()

	// check for empty list
	if len(sigs) == 0 {
		return nil, blsAggregateEmptyListError
	}

	// flatten the shares (required by the C layer)
	flatSigs := make([]byte, 0, signatureLengthBLSBLS12381*len(sigs))
	for i, sig := range sigs {
		if len(sig) != signatureLengthBLSBLS12381 {
			return nil, fmt.Errorf("signature at index %d has an invalid length: %w", i, invalidSignatureError)
		}
		flatSigs = append(flatSigs, sig...)
	}
	aggregatedSig := make([]byte, signatureLengthBLSBLS12381)

	// add the points in the C layer
	result := C.ep_sum_vector_byte(
		(*C.uchar)(&aggregatedSig[0]),
		(*C.uchar)(&flatSigs[0]),
		(C.int)(len(sigs)))

	switch result {
	case valid:
		return aggregatedSig, nil
	case invalid:
		return nil, invalidSignatureError
	default:
		return nil, fmt.Errorf("aggregating signatures failed")
	}
}

// AggregateBLSPrivateKeys aggregates multiple BLS private keys into one.
//
// The order of the keys in the slice does not matter since the aggregation
// is commutative. The slice should not be empty.
// No check is performed on the input private keys.
// Input or output private keys could be equal to the identity element (zero). Note that any
// signature generated by the identity key is invalid (to avoid equivocation issues).
//
// The function returns:
//   - (nil, notBLSKeyError) if at least one key is not of type BLS BLS12-381
//   - (nil, blsAggregateEmptyListError) if no keys are provided (input slice is empty)
//   - (aggregated_key, nil) otherwise
func AggregateBLSPrivateKeys(keys []PrivateKey) (PrivateKey, error) {
	// set BLS context
	blsInstance.reInit()

	// check for empty list
	if len(keys) == 0 {
		return nil, blsAggregateEmptyListError
	}

	scalars := make([]scalar, 0, len(keys))
	for i, sk := range keys {
		skBls, ok := sk.(*prKeyBLSBLS12381)
		if !ok {
			return nil, fmt.Errorf("key at index %d is invalid: %w", i, notBLSKeyError)
		}
		scalars = append(scalars, skBls.scalar)
	}

	var sum scalar
	C.bn_new_wrapper((*C.bn_st)(&sum))
	C.bn_sum_vector((*C.bn_st)(&sum), (*C.bn_st)(&scalars[0]),
		(C.int)(len(scalars)))
	return newPrKeyBLSBLS12381(&sum), nil
}

// AggregateBLSPublicKeys aggregate multiple BLS public keys into one.
//
// The order of the keys in the slice does not matter since the aggregation
// is commutative. The slice should not be empty.
// No check is performed on the input public keys. The input keys are guaranteed by
// the package constructors to be on the G2 subgroup.
// Input or output keys can be equal to the identity key. Note that any
// signature verified against the identity key is invalid (to avoid equivocation issues).
//
// The function returns:
//   - (nil, notBLSKeyError) if at least one key is not of type BLS BLS12-381
//   - (nil, blsAggregateEmptyListError) no keys are provided (input slice is empty)
//   - (aggregated_key, nil) otherwise
func AggregateBLSPublicKeys(keys []PublicKey) (PublicKey, error) {
	// set BLS context
	blsInstance.reInit()

	// check for empty list
	if len(keys) == 0 {
		return nil, blsAggregateEmptyListError
	}

	points := make([]pointG2, 0, len(keys))
	for i, pk := range keys {
		pkBLS, ok := pk.(*pubKeyBLSBLS12381)
		if !ok {
			return nil, fmt.Errorf("key at index %d is invalid: %w", i, notBLSKeyError)
		}
		points = append(points, pkBLS.point)
	}

	var sum pointG2
	C.ep2_sum_vector((*C.ep2_st)(&sum), (*C.ep2_st)(&points[0]),
		(C.int)(len(points)))

	sumKey := newPubKeyBLSBLS12381(&sum)
	return sumKey, nil
}

// IdentityBLSPublicKey returns an identity public key which corresponds to the point
// at infinity in G2 (identity element of G2).
func IdentityBLSPublicKey() PublicKey {
	// set BLS context
	blsInstance.reInit()

	identity := *newPubKeyBLSBLS12381(nil)
	// set the point to infinity
	C.ep2_set_infty((*C.ep2_st)(&identity.point))
	identity.isIdentity = true
	return &identity
}

// RemoveBLSPublicKeys removes multiple BLS public keys from a given (aggregated) public key.
//
// The common use case assumes the aggregated public key was initially formed using
// the keys to be removed (directly or using other aggregated forms). However the function
// can still be called in different use cases.
// The order of the keys to be removed in the slice does not matter since the removal
// is commutative. The slice of keys to be removed can be empty.
// No check is performed on the input public keys. The input keys are guaranteed by the
// package constructors to be on the G2 subgroup.
// Input or output keys can be equal to the identity key.
//
// The function returns:
//   - (nil, notBLSKeyError) if at least one input key is not of type BLS BLS12-381
//   - (remaining_key, nil) otherwise
func RemoveBLSPublicKeys(aggKey PublicKey, keysToRemove []PublicKey) (PublicKey, error) {
	// set BLS context
	blsInstance.reInit()

	aggPKBLS, ok := aggKey.(*pubKeyBLSBLS12381)
	if !ok {
		return nil, notBLSKeyError
	}

	pointsToSubtract := make([]pointG2, 0, len(keysToRemove))
	for i, pk := range keysToRemove {
		pkBLS, ok := pk.(*pubKeyBLSBLS12381)
		if !ok {
			return nil, fmt.Errorf("key at index %d is invalid: %w", i, notBLSKeyError)
		}
		pointsToSubtract = append(pointsToSubtract, pkBLS.point)
	}

	// check for empty list to avoid a cgo edge case
	if len(keysToRemove) == 0 {
		return aggKey, nil
	}

	var resultPoint pointG2
	C.ep2_subtract_vector((*C.ep2_st)(&resultPoint), (*C.ep2_st)(&aggPKBLS.point),
		(*C.ep2_st)(&pointsToSubtract[0]), (C.int)(len(pointsToSubtract)))

	resultKey := newPubKeyBLSBLS12381(&resultPoint)
	return resultKey, nil
}

// VerifyBLSSignatureOneMessage is a multi-signature verification that verifies a
// BLS signature of a single message against multiple BLS public keys.
//
// The input signature could be generated by aggregating multiple signatures of the
// message under multiple private keys. The public keys corresponding to the signing
// private keys are passed as input to this function.
// The caller must make sure the input public keys's proofs of possession have been
// verified prior to calling this function (or each input key is sum of public keys of
// which proofs of possession have been verified).
//
// The input hasher is the same hasher used to generate all initial signatures.
// The order of the public keys in the slice does not matter.
// Membership check is performed on the input signature but is not performed on the input
// public keys (membership is guaranteed by using the package functions).
// If the input public keys add up to the identity public key, the signature is invalid
// to avoid signature equivocation issues.
//
// This is a special case function of VerifyBLSSignatureManyMessages, using a single
// message and hasher.
//
// The function returns:
//   - (false, nilHasherError) if hasher is nil
//   - (false, invalidHasherSizeError) if hasher's output size is not 128 bytes
//   - (false, notBLSKeyError) if at least one key is not of type pubKeyBLSBLS12381
//   - (nil, blsAggregateEmptyListError) if input key slice is empty
//   - (false, error) if an unexpected error occurs
//   - (validity, nil) otherwise
func VerifyBLSSignatureOneMessage(
	pks []PublicKey, s Signature, message []byte, kmac hash.Hasher,
) (bool, error) {
	// public key list must be non empty, this is checked internally by AggregateBLSPublicKeys
	aggPk, err := AggregateBLSPublicKeys(pks)
	if err != nil {
		return false, fmt.Errorf("verify signature one message failed: %w", err)
	}
	return aggPk.Verify(s, message, kmac)
}

// VerifyBLSSignatureManyMessages is a multi-signature verification that verifies a
// BLS signature under multiple messages and public keys.
//
// The input signature could be generated by aggregating multiple signatures of distinct
// messages under distinct private keys. The verification is performed against the message
// at index (i) and the public key at the same index (i) of the input messages and public keys.
// The hasher at index (i) is used to hash the message at index (i).
//
// Since the package only supports the Proof of Possession scheme, the function does not enforce
// input messages to be distinct. Thereore, the caller must make sure the input public keys's proofs
// of possession have been verified prior to calling this function (or each input key is sum of public
// keys of which proofs of possession have been verified).
//
// The verification is optimized to compute one pairing per distinct message, or one pairing
// per distinct key, whatever way offers less pairings calls. If all messages are the same, the
// function has the same behavior as VerifyBLSSignatureOneMessage. If there is one input message and
// input public key, the function has the same behavior as pk.Verify.
// Membership check is performed on the input signature.
// In order to avoid equivocation issues, any identity public key results in the overall
// signature being invalid.
//
// The function returns:
//   - (false, nilHasherError) if a hasher is nil
//   - (false, invalidHasherSizeError) if a hasher's output size is not 128 bytes
//   - (false, notBLSKeyError) if at least one key is not a BLS BLS12-381 key
//   - (false, invalidInputsError) if size of keys is not matching the size of messages and hashers
//   - (false, blsAggregateEmptyListError) if input key slice `pks` is empty
//   - (false, error) if an unexpected error occurs
//   - (validity, nil) otherwise
func VerifyBLSSignatureManyMessages(
	pks []PublicKey, s Signature, messages [][]byte, kmac []hash.Hasher,
) (bool, error) {
	// set BLS context
	blsInstance.reInit()

	// check signature length
	if len(s) != signatureLengthBLSBLS12381 {
		return false, nil
	}
	// check the list lengths
	if len(pks) == 0 {
		return false, fmt.Errorf("invalid list of public keys: %w", blsAggregateEmptyListError)
	}
	if len(pks) != len(messages) || len(kmac) != len(messages) {
		return false, invalidInputsErrorf(
			"input lists must be equal, messages are %d, keys are %d, hashers are %d",
			len(messages),
			len(pks),
			len(kmac))
	}

	// compute the hashes
	hashes := make([][]byte, 0, len(messages))
	for i, k := range kmac {
		if err := checkBLSHasher(k); err != nil {
			return false, fmt.Errorf("hasher at index %d is invalid: %w ", i, err)
		}
		hashes = append(hashes, k.ComputeHash(messages[i]))
	}

	// two maps to count the type (keys or messages) with the least distinct elements.
	// mapPerHash maps hashes to keys while mapPerPk maps keys to hashes.
	// The comparison of the maps length minimizes the number of pairings to
	// compute by aggregating either public keys or the message hashes in
	// the verification equation.
	mapPerHash := make(map[string][]pointG2)
	mapPerPk := make(map[pointG2][][]byte)
	// Note: mapPerPk is using a cgo structure as map keys which may lead to 2 equal public keys
	// being considered distinct. This does not make the verification equation wrong but leads to
	// computing extra pairings. This case is considered unlikely to happen since a caller is likely
	// to use the same struct for a same public key.
	// One way to fix this is to use the public key encoding as the map keys and store the "pointG2"
	// structure with the map value, which adds more complexity and processing time.

	// fill the 2 maps
	for i, pk := range pks {
		pkBLS, ok := pk.(*pubKeyBLSBLS12381)
		if !ok {
			return false, fmt.Errorf(
				"public key at index %d is invalid: %w",
				i, notBLSKeyError)
		}
		// check identity check
		if pkBLS.isIdentity {
			return false, nil
		}

		mapPerHash[string(hashes[i])] = append(mapPerHash[string(hashes[i])], pkBLS.point)
		mapPerPk[pkBLS.point] = append(mapPerPk[pkBLS.point], hashes[i])
	}

	var verif (C.int)
	//compare the 2 maps for the shortest length
	if len(mapPerHash) < len(mapPerPk) {
		// aggregate keys per distinct hashes
		// using the linearity of the pairing on the G2 variables.
		flatDistinctHashes := make([]byte, 0)
		lenHashes := make([]uint32, 0)
		pkPerHash := make([]uint32, 0, len(mapPerHash))
		allPks := make([]pointG2, 0)
		for hash, pksVal := range mapPerHash {
			flatDistinctHashes = append(flatDistinctHashes, []byte(hash)...)
			lenHashes = append(lenHashes, uint32(len([]byte(hash))))
			pkPerHash = append(pkPerHash, uint32(len(pksVal)))
			allPks = append(allPks, pksVal...)
		}
		verif = C.bls_verifyPerDistinctMessage(
			(*C.uchar)(&s[0]),
			(C.int)(len(mapPerHash)),
			(*C.uchar)(&flatDistinctHashes[0]),
			(*C.uint32_t)(&lenHashes[0]),
			(*C.uint32_t)(&pkPerHash[0]),
			(*C.ep2_st)(&allPks[0]),
		)

	} else {
		// aggregate hashes per distinct key
		// using the linearity of the pairing on the G1 variables.
		distinctPks := make([]pointG2, 0, len(mapPerPk))
		hashPerPk := make([]uint32, 0, len(mapPerPk))
		flatHashes := make([]byte, 0)
		lenHashes := make([]uint32, 0)
		for pk, hashesVal := range mapPerPk {
			distinctPks = append(distinctPks, pk)
			hashPerPk = append(hashPerPk, uint32(len(hashesVal)))
			for _, h := range hashesVal {
				flatHashes = append(flatHashes, h...)
				lenHashes = append(lenHashes, uint32(len(h)))
			}
		}

		verif = C.bls_verifyPerDistinctKey(
			(*C.uchar)(&s[0]),
			(C.int)(len(mapPerPk)),
			(*C.ep2_st)(&distinctPks[0]),
			(*C.uint32_t)(&hashPerPk[0]),
			(*C.uchar)(&flatHashes[0]),
			(*C.uint32_t)(&lenHashes[0]))
	}

	switch verif {
	case invalid:
		return false, nil
	case valid:
		return true, nil
	default:
		return false, fmt.Errorf("signature verification failed")
	}
}

// BatchVerifyBLSSignaturesOneMessage is a batch verification of multiple
// BLS signatures of a single message against multiple BLS public keys that
// is faster than verifying the signatures one by one.
//
// Each signature at index (i) of the input signature slice is verified against
// the public key of the same index (i) in the input key slice.
// The input hasher is the same used to generate all signatures.
// The returned boolean slice is a slice so that the value at index (i) is true
// if signature (i) verifies against public key (i), and false otherwise.
//
// The caller must make sure the input public keys's proofs of possession have been
// verified prior to calling this function (or each input key is sum of public
// keys of which proofs of possession have been verified).
//
// Membership checks are performed on the input signatures but are not performed
// on the input public keys (which are guaranteed by the package to be on the correct
// G2 subgroup).
// In order to avoid equivocation issues, any identity public key results in the corresponding
// signature being invalid.
//
// The function returns:
//   - ([]false, nilHasherError) if a hasher is nil
//   - ([]false, invalidHasherSizeError) if a hasher's output size is not 128 bytes
//   - ([]false, notBLSKeyError) if at least one key is not of type BLS BLS12-381
//   - ([]false, invalidInputsError) if size of keys is not matching the size of signatures
//   - ([]false, blsAggregateEmptyListError) if input key slice is empty
//   - ([]false, error) if an unexpected error occurs
//   - ([]validity, nil) otherwise
func BatchVerifyBLSSignaturesOneMessage(
	pks []PublicKey, sigs []Signature, message []byte, kmac hash.Hasher,
) ([]bool, error) {
	// set BLS context
	blsInstance.reInit()

	// empty list check
	if len(pks) == 0 {
		return []bool{}, fmt.Errorf("invalid list of public keys: %w", blsAggregateEmptyListError)
	}

	if len(pks) != len(sigs) {
		return []bool{}, invalidInputsErrorf(
			"keys length %d and signatures length %d are mismatching",
			len(pks),
			len(sigs))
	}

	verifBool := make([]bool, len(sigs))
	if err := checkBLSHasher(kmac); err != nil {
		return verifBool, err
	}

	// an invalid signature with an incorrect header but correct length
	invalidSig := make([]byte, signatureLengthBLSBLS12381)
	invalidSig[0] = invalidBLSSignatureHeader // incorrect header

	// flatten the shares (required by the C layer)
	flatSigs := make([]byte, 0, signatureLengthBLSBLS12381*len(sigs))
	pkPoints := make([]pointG2, 0, len(pks))

	for i, pk := range pks {
		pkBLS, ok := pk.(*pubKeyBLSBLS12381)
		if !ok {
			return verifBool, fmt.Errorf("key at index %d is invalid: %w", i, notBLSKeyError)
		}
		pkPoints = append(pkPoints, pkBLS.point)

		if len(sigs[i]) != signatureLengthBLSBLS12381 || pkBLS.isIdentity {
			// force the signature to be invalid by replacing it with an invalid array
			// that fails the deserialization in C.ep_read_bin_compact
			flatSigs = append(flatSigs, invalidSig...)
		} else {
			flatSigs = append(flatSigs, sigs[i]...)
		}
	}

	// hash the input to 128 bytes
	h := kmac.ComputeHash(message)
	verifInt := make([]byte, len(verifBool))

	C.bls_batchVerify(
		(C.int)(len(verifInt)),
		(*C.uchar)(&verifInt[0]),
		(*C.ep2_st)(&pkPoints[0]),
		(*C.uchar)(&flatSigs[0]),
		(*C.uchar)(&h[0]),
		(C.int)(len(h)),
	)

	for i, v := range verifInt {
		if (C.int)(v) != valid && (C.int)(v) != invalid {
			return verifBool, fmt.Errorf("batch verification failed")
		}
		verifBool[i] = ((C.int)(v) == valid)
	}

	return verifBool, nil
}

// blsAggregateEmptyListError is returned when a list of BLS objects (e.g. signatures or keys)
// is empty or nil and thereby represents an invalid input.
var blsAggregateEmptyListError = errors.New("list cannot be empty")

// IsBLSAggregateEmptyListError checks if err is an `blsAggregateEmptyListError`.
// blsAggregateEmptyListError is returned when a BLS aggregation function is called with
// an empty list which is not allowed in some aggregation cases to avoid signature equivocation
// issues.
func IsBLSAggregateEmptyListError(err error) bool {
	return errors.Is(err, blsAggregateEmptyListError)
}

// notBLSKeyError is returned when a private or public key
// used is not a BLS on BLS12 381 key.
var notBLSKeyError = errors.New("input key has to be a BLS on BLS12-381 key")

// IsNotBLSKeyError checks if err is an `notBLSKeyError`.
// notBLSKeyError is returned when a private or public key
// used is not a BLS on BLS12 381 key.
func IsNotBLSKeyError(err error) bool {
	return errors.Is(err, notBLSKeyError)
}

// invalidSignatureError is returned when a signature input does not serialize to a
// valid element on E1 of the BLS12-381 curve (but without checking the element is on subgroup G1).
var invalidSignatureError = errors.New("input signature does not deserialize to an E1 element")

// IsInvalidSignatureError checks if err is an `invalidSignatureError`
// invalidSignatureError is returned when a signature input does not serialize to a
// valid element on E1 of the BLS12-381 curve (but without checking the element is on subgroup G1).
func IsInvalidSignatureError(err error) bool {
	return errors.Is(err, invalidSignatureError)
}
