//go:build relic
// +build relic

package crypto

// BLS signature scheme implementation using BLS12-381 curve
// ([zcash]https://electriccoin.co/blog/new-snark-curve/)
// Pairing, ellipic curve and modular arithmetic is using Relic library.
// This implementation does not include any security against side-channel attacks.

// existing features:
//  - the implementation variant is minimal-signature-size signatures:
//    shorter signatures in G1, longer public keys in G2
//  - serialization of points on G1 and G2 is compressed ([zcash]
//     https://www.ietf.org/archive/id/draft-irtf-cfrg-pairing-friendly-curves-08.html#name-zcash-serialization-format-)
//  - hashing to curve uses the Simplified SWU map-to-curve
//    (https://datatracker.ietf.org/doc/html/draft-irtf-cfrg-hash-to-curve-14#section-6.6.3)
//  - expanding the message in hash-to-curve uses a cSHAKE-based KMAC128 with a domain separation tag.
//    KMAC128 serves as an expand_message_xof function.
//  - this results in the full ciphersuite BLS_SIG_BLS12381G1_XOF:KMAC128_SSWU_RO_POP_ for signatures
//    and BLS_POP_BLS12381G1_XOF:KMAC128_SSWU_RO_POP_ for proofs of possession.
//  - signature verification checks the membership of signature in G1.
//  - the public key membership check in G2 is implemented separately from the signature verification.
//  - membership check in G1 is implemented using fast Bowe's check (to be updated to Scott's check).
//  - membership check in G2 is using a simple scalar multiplication with the group order (to be updated to Scott's check).
//  - multi-signature tools are defined in bls_multisg.go
//  - SPoCK scheme based on BLS: verifies two signatures have been generated from the same message,
//    that is unknown to the verifier.

// future features:
//  - membership checks G2 using Bowe's method (https://eprint.iacr.org/2019/814.pdf)
//  - implement a G1/G2 swap (signatures on G2 and public keys on G1)

// #cgo CFLAGS: -g -Wall -std=c99 -I${SRCDIR}/ -I${SRCDIR}/relic/build/include
// #cgo LDFLAGS: -L${SRCDIR}/relic/build/lib -l relic_s
// #include "bls_include.h"
import "C"

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/crypto/hash"
)

// blsBLS12381Algo, embeds SignAlgo
type blsBLS12381Algo struct {
	// points to Relic context of BLS12-381 with all the parameters
	context ctx
	// the signing algo and parameters
	algo SigningAlgorithm
}

//  BLS context on the BLS 12-381 curve
var blsInstance *blsBLS12381Algo

// NewExpandMsgXOFKMAC128 returns a new expand_message_xof instance for
// the hash-to-curve function, hashing data to G1 on BLS12 381.
// This instance must only be used to generate signatures (and not PoP),
// because the internal ciphersuite is customized for signatures. It
// is guaranteed to be different than the expand_message_xof instance used
// to generate proofs of possession.
//
// KMAC128 is used as the underligned extendable-output function (xof)
// as required by https://datatracker.ietf.org/doc/html/draft-irtf-cfrg-hash-to-curve-14#section-5.4.4.
//
// `domainTag` is a domain separation tag that defines the protocol and its subdomain. Such tag should be of the
// format: <protocol>-V<xx>-CS<yy>-with- where <protocol> is the name of the protocol, <xx> the protocol
// version number and <yy> the index of the ciphersuite in the protocol.
// The function suffixes the given `domainTag` by the BLS ciphersuite supported by the library.
//
// The returned instance is a `Hasher` and can be used to generate BLS signatures
// with the `Sign` method.
func NewExpandMsgXOFKMAC128(domainTag string) hash.Hasher {
	// application tag is guaranteed to be different than the tag used
	// to generate proofs of possession
	// postfix the domain tag with the BLS ciphersuite
	key := domainTag + blsSigCipherSuite
	return internalExpandMsgXOFKMAC128(key)
}

// returns an expand_message_xof instance for
// the hash-to-curve function, hashing data to G1 on BLS12 381.
// The key is used as a customizer rather than a MAC key.
func internalExpandMsgXOFKMAC128(key string) hash.Hasher {
	// UTF-8 is used by Go to convert strings into bytes.
	// UTF-8 is a non-ambiguous encoding as required by draft-irtf-cfrg-hash-to-curve
	// (similarly to the recommended ASCII).

	// blsKMACFunction is the customizer used for KMAC in BLS
	const blsKMACFunction = "H2C"
	// the error is ignored as the parameter lengths are chosen to be in the correct range for kmac
	// (tested by TestBLSBLS12381Hasher)
	kmac, _ := hash.NewKMAC_128([]byte(key), []byte(blsKMACFunction), expandMsgOutput)
	return kmac
}

// Sign signs an array of bytes using the private key
//
// Signature is compressed [zcash]
// https://github.com/zkcrypto/pairing/blob/master/src/bls12_381/README.md#serialization
// The private key is read only.
// If the hasher used is KMAC128, the hasher is read only.
// It is recommended to use Sign with the hasher from NewExpandMsgXOFKMAC128. If not, the hasher used
// must expand the message to 1024 bits. It is also recommended to use a hasher
// with a domain separation tag.
func (sk *PrKeyBLSBLS12381) Sign(data []byte, kmac hash.Hasher) (Signature, error) {
	if kmac == nil {
		return nil, invalidInputsErrorf("hasher is empty")
	}
	// check hasher output size
	if kmac.Size() != expandMsgOutput {
		return nil, invalidInputsErrorf(
			"hasher with %d output byte size is required, got hasher with size %d",
			expandMsgOutput,
			kmac.Size())
	}
	// hash the input to 128 bytes
	h := kmac.ComputeHash(data)

	// set BLS context
	blsInstance.reInit()

	s := make([]byte, SignatureLenBLSBLS12381)
	C.bls_sign((*C.uchar)(&s[0]),
		(*C.bn_st)(&sk.scalar),
		(*C.uchar)(&h[0]),
		(C.int)(len(h)))
	return s, nil
}

// Verify verifies a signature of a byte array using the public key and the input hasher.
//
// If the input signature slice has an invalid length or fails to deserialize into a curve
// point, the function returns false without an error.
//
// The function assumes the public key is in the valid G2 subgroup as it is
// either generated by the library or read through the DecodePublicKey function,
// which includes a validity check.
// The signature membership check in G1 is included in the verifcation.
//
// If the hasher used is KMAC128, the hasher is read only.
func (pk *PubKeyBLSBLS12381) Verify(s Signature, data []byte, kmac hash.Hasher) (bool, error) {
	if len(s) != signatureLengthBLSBLS12381 {
		return false, nil
	}

	if kmac == nil {
		return false, invalidInputsErrorf("hasher is empty")
	}
	// check hasher output size
	if kmac.Size() != expandMsgOutput {
		return false, invalidInputsErrorf(
			"hasher with at least %d output byte size is required, got hasher with size %d",
			expandMsgOutput,
			kmac.Size())
	}

	// hash the input to 128 bytes
	h := kmac.ComputeHash(data)

	// intialize BLS context
	blsInstance.reInit()

	verif := C.bls_verify((*C.ep2_st)(&pk.point),
		(*C.uchar)(&s[0]),
		(*C.uchar)(&h[0]),
		(C.int)(len(h)))

	switch verif {
	case invalid:
		return false, nil
	case valid:
		return true, nil
	default:
		return false, fmt.Errorf("signature verification failed")
	}
}

// generatePrivateKey generates a private key for BLS on BLS12-381 curve.
// The minimum size of the input seed is 48 bytes.
//
// It is recommended to use a secure crypto RNG to generate the seed.
// The seed must have enough entropy and should be sampled uniformly at random.
func (a *blsBLS12381Algo) generatePrivateKey(seed []byte) (PrivateKey, error) {
	if len(seed) < KeyGenSeedMinLenBLSBLS12381 || len(seed) > KeyGenSeedMaxLenBLSBLS12381 {
		return nil, invalidInputsErrorf(
			"seed length should be between %d and %d bytes",
			KeyGenSeedMinLenBLSBLS12381,
			KeyGenSeedMaxLenBLSBLS12381)
	}

	sk := newPrKeyBLSBLS12381(nil)

	// maps the seed to a private key
	// error is not checked as it is guaranteed to be nil
	mapToZr(&sk.scalar, seed)
	return sk, nil
}

const invalidBLSSignatureHeader = byte(0xE0)

// BLSInvalidSignature returns an invalid signature that fails when verified
// with any message and public key.
//
// The signature bytes represent an invalid serialization of a point which
// makes the verification fail early. The verification would return (false, nil).
func BLSInvalidSignature() Signature {
	signature := make([]byte, SignatureLenBLSBLS12381)
	signature[0] = invalidBLSSignatureHeader // invalid header as per C.ep_read_bin_compact
	return signature
}

// decodePrivateKey decodes a slice of bytes into a private key.
// This function checks the scalar is less than the group order
func (a *blsBLS12381Algo) decodePrivateKey(privateKeyBytes []byte) (PrivateKey, error) {
	if len(privateKeyBytes) != prKeyLengthBLSBLS12381 {
		return nil, invalidInputsErrorf("input length must be %d, got %d",
			prKeyLengthBLSBLS12381, len(privateKeyBytes))
	}
	sk := newPrKeyBLSBLS12381(nil)

	readScalar(&sk.scalar, privateKeyBytes)
	if C.check_membership_Zr((*C.bn_st)(&sk.scalar)) == valid {
		return sk, nil
	}

	return nil, invalidInputsErrorf("the private key is not a valid BLS12-381 curve key")
}

// decodePublicKey decodes a slice of bytes into a public key.
// This function includes a membership check in G2 and rejects the infinity point.
func (a *blsBLS12381Algo) decodePublicKey(publicKeyBytes []byte) (PublicKey, error) {
	if len(publicKeyBytes) != pubKeyLengthBLSBLS12381 {
		return nil, invalidInputsErrorf("input length must be %d, got %d",
			pubKeyLengthBLSBLS12381, len(publicKeyBytes))
	}
	var pk PubKeyBLSBLS12381
	err := readPointG2(&pk.point, publicKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("decode public key failed %w", err)
	}
	if !pk.point.checkValidPublicKeyPoint() {
		return nil, invalidInputsErrorf("input key is infinity or does not encode a BLS12-381 point in the valid group")
	}
	return &pk, nil
}

// decodePublicKeyCompressed decodes a slice of bytes into a public key.
// since we use the compressed representation by default, this checks the default and delegates to decodePublicKeyCompressed
func (a *blsBLS12381Algo) decodePublicKeyCompressed(publicKeyBytes []byte) (PublicKey, error) {
	if serializationG2 != compressed {
		panic("library is not configured to use compressed public key serialization")
	}
	return a.decodePublicKey(publicKeyBytes)
}

// PrKeyBLSBLS12381 is the private key of BLS using BLS12_381, it implements PrivateKey
type PrKeyBLSBLS12381 struct {
	// public key
	pk *PubKeyBLSBLS12381
	// private key data
	scalar scalar
}

// newPrKeyBLSBLS12381 creates a new BLS private key with the given scalar.
// If no scalar is provided, the function allocates an
// empty scalar.
func newPrKeyBLSBLS12381(x *scalar) *PrKeyBLSBLS12381 {
	var sk PrKeyBLSBLS12381
	if x == nil {
		// initialize the scalar
		C.bn_new_wrapper((*C.bn_st)(&sk.scalar))
	} else {
		// set the scalar
		sk.scalar = *x
	}
	// the embedded public key is only computed when needed
	return &sk
}

// Algorithm returns the Signing Algorithm
func (sk *PrKeyBLSBLS12381) Algorithm() SigningAlgorithm {
	return BLSBLS12381
}

// Size returns the private key lengh in bytes
func (sk *PrKeyBLSBLS12381) Size() int {
	return PrKeyLenBLSBLS12381
}

// computePublicKey generates the public key corresponding to
// the input private key. The function makes sure the piblic key
// is valid in G2.
func (sk *PrKeyBLSBLS12381) computePublicKey() {
	var newPk PubKeyBLSBLS12381
	// compute public key pk = g2^sk
	generatorScalarMultG2(&(newPk.point), &(sk.scalar))
	sk.pk = &newPk
}

// PublicKey returns the public key corresponding to the private key
func (sk *PrKeyBLSBLS12381) PublicKey() PublicKey {
	if sk.pk != nil {
		return sk.pk
	}
	sk.computePublicKey()
	return sk.pk
}

// Encode returns a byte encoding of the private key.
// The encoding is a raw encoding in big endian padded to the group order
func (a *PrKeyBLSBLS12381) Encode() []byte {
	dest := make([]byte, prKeyLengthBLSBLS12381)
	writeScalar(dest, &a.scalar)
	return dest
}

// Equals checks is two public keys are equal.
func (sk *PrKeyBLSBLS12381) Equals(other PrivateKey) bool {
	otherBLS, ok := other.(*PrKeyBLSBLS12381)
	if !ok {
		return false
	}
	return sk.scalar.equals(&otherBLS.scalar)
}

// String returns the hex string representation of the key.
func (sk *PrKeyBLSBLS12381) String() string {
	return fmt.Sprintf("%#x", sk.Encode())
}

// PubKeyBLSBLS12381 is the public key of BLS using BLS12_381,
// it implements PublicKey
type PubKeyBLSBLS12381 struct {
	// public key data
	point pointG2
}

// newPubKeyBLSBLS12381 creates a new BLS public key with the given point.
// If no scalar is provided, the function allocates an
// empty scalar.
func newPubKeyBLSBLS12381(p *pointG2) *PubKeyBLSBLS12381 {
	if p != nil {
		return &PubKeyBLSBLS12381{
			point: *p,
		}
	}
	return &PubKeyBLSBLS12381{}
}

// Algorithm returns the Signing Algorithm
func (pk *PubKeyBLSBLS12381) Algorithm() SigningAlgorithm {
	return BLSBLS12381
}

// Size returns the public key lengh in bytes
func (pk *PubKeyBLSBLS12381) Size() int {
	return PubKeyLenBLSBLS12381
}

// Encode returns a byte encoding of the public key.
// The encoding is a compressed encoding of the point
// [zcash] https://github.com/zkcrypto/pairing/blob/master/src/bls12_381/README.md#serialization
func (a *PubKeyBLSBLS12381) EncodeCompressed() []byte {
	if serializationG2 != compressed {
		panic("library is not configured to use compressed public key serialization")
	}
	return a.Encode()
}

// Encode returns a byte encoding of the public key.
// Since we use a compressed encoding by default, this delegates to EncodeCompressed
func (a *PubKeyBLSBLS12381) Encode() []byte {
	dest := make([]byte, pubKeyLengthBLSBLS12381)
	writePointG2(dest, &a.point)
	return dest
}

// Equals checks is two public keys are equal
func (pk *PubKeyBLSBLS12381) Equals(other PublicKey) bool {
	otherBLS, ok := other.(*PubKeyBLSBLS12381)
	if !ok {
		return false
	}
	return pk.point.equals(&otherBLS.point)
}

// String returns the hex string representation of the key.
func (pk *PubKeyBLSBLS12381) String() string {
	return fmt.Sprintf("%#x", pk.Encode())
}

// Get Macro definitions from the C layer as Cgo does not export macros
var signatureLengthBLSBLS12381 = int(C.get_signature_len())
var pubKeyLengthBLSBLS12381 = int(C.get_pk_len())
var prKeyLengthBLSBLS12381 = int(C.get_sk_len())

// init sets the context of BLS12-381 curve
func (a *blsBLS12381Algo) init() error {
	// initializes relic context and sets the B12_381 parameters
	if err := a.context.initContext(); err != nil {
		return err
	}

	// compare the Go and C layer constants as a sanity check
	if signatureLengthBLSBLS12381 != SignatureLenBLSBLS12381 ||
		pubKeyLengthBLSBLS12381 != PubKeyLenBLSBLS12381 ||
		prKeyLengthBLSBLS12381 != PrKeyLenBLSBLS12381 {
		return errors.New("BLS-12381 length settings in Go and C are not consistent, check hardcoded lengths and compressions")
	}
	return nil
}

// set the context of BLS 12-381 curve in the lower C and Relic layers assuming the context
// was previously initialized with a call to init().
//
// If the implementation evolves to support multiple contexts,
// reinit should be called at every blsBLS12381Algo operation.
func (a *blsBLS12381Algo) reInit() {
	a.context.setContext()
}

// checkValidPublicKeyPoint checks whether the input point is a valid public key for BLS
// on the BLS12-381 curve, considering public keys are in G2.
// It returns true if the public key is non-infinity and on the correct subgroup of the curve
// and false otherwise.
//
// It is necessary to run this test once for every public key before
// it is used to verify BLS signatures. The library calls this function whenever
// it imports a key through the function DecodePublicKey.
// The validity check is separated from the signature verification to optimize
// multiple verification calls using the same public key.
func (pk *pointG2) checkValidPublicKeyPoint() bool {
	// check point is non-infinity
	verif := C.ep2_is_infty((*C.ep2_st)(pk))
	if verif != valid {
		return false
	}

	// membership check in G2
	verif = C.check_membership_G2((*C.ep2_st)(pk))
	return verif == valid
}

// This is only a TEST/DEBUG/BENCH function.
// It returns the hash to G1 point from a slice of 128 bytes
func mapToG1(data []byte) *pointG1 {
	l := len(data)
	var h pointG1
	C.map_to_G1((*C.ep_st)(&h), (*C.uchar)(&data[0]), (C.int)(l))
	return &h
}

// This is only a TEST function.
// signWithXMDSHA256 signs a message using XMD_SHA256 as a hash to field.
//
// The function is in this file because cgo can't be used in go test files.
// TODO: implement a hasher for XMD SHA256 and use the `Sign` function.
func (sk *PrKeyBLSBLS12381) signWithXMDSHA256(data []byte) Signature {

	dst := []byte("BLS_SIG_BLS12381G1_XMD:SHA-256_SSWU_RO_NUL_")
	hash := make([]byte, expandMsgOutput)
	// XMD using SHA256
	C.xmd_sha256((*C.uchar)(&hash[0]),
		(C.int)(expandMsgOutput),
		(*C.uchar)(&data[0]), (C.int)(len(data)),
		(*C.uchar)(&dst[0]), (C.int)(len(dst)))

	// sign the hash
	s := make([]byte, SignatureLenBLSBLS12381)
	C.bls_sign((*C.uchar)(&s[0]),
		(*C.bn_st)(&sk.scalar),
		(*C.uchar)(&hash[0]),
		(C.int)(len(hash)))
	return s
}
