package crypto

// BLS signature scheme implementation using the BLS12-381 curve
// ([zcash]https://electriccoin.co/blog/new-snark-curve/).
// Pairing, ellipic curve and modular arithmetic are using [BLST](https://github.com/supranational/blst/tree/master/src)
// tools underneath.
// This implementation does not include security against side-channel or fault attacks.

// Existing features:
//  - the implementation variant is minimal-signature-size:
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
//  - multi-signature tools are defined in bls_multisg.go
//  - SPoCK scheme based on BLS: verifies two signatures are generated from the same message,
//    even though the message is unknown to the verifier.

// #include "bls_include.h"
import "C"

import (
	"bytes"
	"crypto/sha256"
	"fmt"

	"golang.org/x/crypto/hkdf"

	"github.com/onflow/flow-go/crypto/hash"
)

const (
	// SignatureLenBLSBLS12381 is the serialization size of a `G_1` element.
	SignatureLenBLSBLS12381 = g1BytesLen
	// PubKeyLenBLSBLS12381 is the serialization size of a `G_2` element.
	PubKeyLenBLSBLS12381 = g2BytesLen
	// PrKeyLenBLSBLS12381 is the serialization size of a `F_r` element,
	// where `r` is the order of `G_1` and `G_2`.
	PrKeyLenBLSBLS12381 = frBytesLen

	// Hash to curve params
	// hash to curve suite ID of the form : CurveID_ || HashID_ || MapID_ || encodingVariant_
	h2cSuiteID = "BLS12381G1_XOF:KMAC128_SSWU_RO_"
	// scheme implemented as a countermasure for rogue attacks of the form : SchemeTag_
	schemeTag = "POP_"
	// Cipher suite used for BLS signatures of the form : BLS_SIG_ || h2cSuiteID || SchemeTag_
	blsSigCipherSuite = "BLS_SIG_" + h2cSuiteID + schemeTag
	// Cipher suite used for BLS PoP of the form : BLS_POP_ || h2cSuiteID || SchemeTag_
	// The PoP cipher suite is guaranteed to be different than all signature ciphersuites
	blsPOPCipherSuite = "BLS_POP_" + h2cSuiteID + schemeTag
	// expandMsgOutput is the output length of the expand_message step as required by the
	// hash_to_curve algorithm (and the map to G1 step).
	expandMsgOutput = int(C.MAP_TO_G1_INPUT_LEN)
)

// blsBLS12381Algo, embeds SignAlgo
type blsBLS12381Algo struct {
	// the signing algo and parameters
	algo SigningAlgorithm
}

// BLS context on the BLS 12-381 curve
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
	// blsKMACFunction is the customizer used for KMAC in BLS
	const blsKMACFunction = "H2C"
	// the error is ignored as the parameter lengths are chosen to be in the correct range for kmac
	// (tested by TestBLSBLS12381Hasher)
	kmac, _ := hash.NewKMAC_128([]byte(key), []byte(blsKMACFunction), expandMsgOutput)
	return kmac
}

// checkBLSHasher asserts that the given `hasher` is not nil and
// has an output size of `expandMsgOutput`. Otherwise an error is returned:
//   - nilHasherError if the hasher is nil
//   - invalidHasherSizeError if the hasher's output size is not `expandMsgOutput` (128 bytes)
func checkBLSHasher(hasher hash.Hasher) error {
	if hasher == nil {
		return nilHasherError
	}
	if hasher.Size() != expandMsgOutput {
		return invalidHasherSizeErrorf("hasher's size needs to be %d, got %d", expandMsgOutput, hasher.Size())
	}
	return nil
}

// Sign signs an array of bytes using the private key
//
// Signature is compressed [zcash]
// https://www.ietf.org/archive/id/draft-irtf-cfrg-pairing-friendly-curves-08.html#name-zcash-serialization-format-
// The private key is read only.
// If the hasher used is KMAC128, the hasher is read only.
// It is recommended to use Sign with the hasher from NewExpandMsgXOFKMAC128. If not, the hasher used
// must expand the message to 1024 bits. It is also recommended to use a hasher
// with a domain separation tag.
//
// The function returns:
//   - (false, nilHasherError) if a hasher is nil
//   - (false, invalidHasherSizeError) if a hasher's output size is not 128 bytes
//   - (signature, nil) otherwise
func (sk *prKeyBLSBLS12381) Sign(data []byte, kmac hash.Hasher) (Signature, error) {
	// sanity check of input hasher
	err := checkBLSHasher(kmac)
	if err != nil {
		return nil, err
	}

	// hash the input to 128 bytes
	h := kmac.ComputeHash(data)

	s := make([]byte, SignatureLenBLSBLS12381)
	C.bls_sign((*C.uchar)(&s[0]),
		(*C.Fr)(&sk.scalar),
		(*C.uchar)(&h[0]),
		(C.int)(len(h)))
	return s, nil
}

// Verify verifies a signature of a byte array using the public key and the input hasher.
//
// If the input signature slice has an invalid length or fails to deserialize into a curve
// subgroup point, the function returns false without an error.
//
// The function assumes the public key is in the valid G2 subgroup because
// all the package functions generating a BLS `PublicKey` include a G2-membership check.
// The public keys are not guaranteed to be non-identity, and therefore the function
// includes an identity comparison. Verifications against an identity public key
// are invalid to avoid equivocation issues.
// The signature membership check in G1 is included in the verification.
//
// If the hasher used is ExpandMsgXOFKMAC128, the hasher is read only.
//
// The function returns:
//   - (false, nilHasherError) if a hasher is nil
//   - (false, invalidHasherSizeError) if a hasher's output size is not 128 bytes
//   - (false, error) if an unexpected error occurs
//   - (validity, nil) otherwise
func (pk *pubKeyBLSBLS12381) Verify(s Signature, data []byte, kmac hash.Hasher) (bool, error) {
	// check of input hasher
	err := checkBLSHasher(kmac)
	if err != nil {
		return false, err
	}

	if len(s) != SignatureLenBLSBLS12381 {
		return false, nil
	}

	// hash the input to 128 bytes
	h := kmac.ComputeHash(data)

	// check for identity public key
	if pk.isIdentity {
		return false, nil
	}

	verif := C.bls_verify((*C.E2)(&pk.point),
		(*C.uchar)(&s[0]),
		(*C.uchar)(&h[0]),
		(C.int)(len(h)))

	switch verif {
	case invalid:
		return false, nil
	case valid:
		return true, nil
	default:
		return false, fmt.Errorf("signature verification failed: code %d", verif)
	}
}

// IsBLSSignatureIdentity checks whether the input signature is
// the identity signature (point at infinity in G1).
//
// An identity signature is always an invalid signature even when
// verified against the identity public key.
// This identity check is useful when an aggregated signature is
// suspected to be equal to identity, which avoids failing the aggregated
// signature verification.
func IsBLSSignatureIdentity(s Signature) bool {
	return bytes.Equal(s, g1Serialization)
}

// generatePrivateKey deterministically generates a private key for BLS on BLS12-381 curve.
// The minimum size of the input seed is 32 bytes.
//
// It is recommended to use a secure crypto RNG to generate the seed.
// Otherwise, the seed must have enough entropy.
//
// The generated private key (resp. its corresponding public key) is guaranteed
// to not be equal to the identity element of Z_r (resp. G2).
func (a *blsBLS12381Algo) generatePrivateKey(ikm []byte) (PrivateKey, error) {
	if len(ikm) < KeyGenSeedMinLen || len(ikm) > KeyGenSeedMaxLen {
		return nil, invalidInputsErrorf(
			"seed length should be at least %d bytes and at most %d bytes",
			KeyGenSeedMinLen, KeyGenSeedMaxLen)
	}

	// HKDF parameters

	// use SHA2-256 as the building block H in HKDF
	hashFunction := sha256.New
	// salt = H(UTF-8("BLS-SIG-KEYGEN-SALT-")) as per draft-irtf-cfrg-bls-signature-05 section 2.3.
	saltString := "BLS-SIG-KEYGEN-SALT-"
	hasher := hashFunction()
	hasher.Write([]byte(saltString))
	salt := make([]byte, hasher.Size())
	hasher.Sum(salt[:0])

	// L is the OKM length
	// L = ceil((3 * ceil(log2(r))) / 16) which makes L (security_bits/8)-larger than r size
	okmLength := (3 * frBytesLen) / 2

	// HKDF secret = IKM || I2OSP(0, 1)
	secret := make([]byte, len(ikm)+1)
	copy(secret, ikm)
	defer overwrite(secret) // overwrite secret
	// HKDF info = key_info || I2OSP(L, 2)
	keyInfo := "" // use empty key diversifier. TODO: update header to accept input identifier
	info := append([]byte(keyInfo), byte(okmLength>>8), byte(okmLength))

	sk := newPrKeyBLSBLS12381(nil)
	for {
		// instantiate HKDF and extract L bytes
		reader := hkdf.New(hashFunction, secret, salt, info)
		okm := make([]byte, okmLength)
		n, err := reader.Read(okm)
		if err != nil || n != okmLength {
			return nil, fmt.Errorf("key generation failed because of the HKDF reader, %d bytes were read: %w",
				n, err)
		}
		defer overwrite(okm) // overwrite okm

		// map the bytes to a private key using modular reduction
		// SK = OS2IP(OKM) mod r
		isZero := mapToFr(&sk.scalar, okm)
		if !isZero {
			return sk, nil
		}

		// update salt = H(salt)
		hasher.Reset()
		hasher.Write(salt)
		salt = hasher.Sum(salt[:0])
	}
}

const invalidBLSSignatureHeader = byte(0xE0)

// BLSInvalidSignature returns an invalid signature that fails when verified
// with any message and public key, which can be used for testing.
//
// The signature bytes represent an invalid serialization of a point which
// makes the verification fail early. The verification would return (false, nil).
func BLSInvalidSignature() Signature {
	signature := make([]byte, SignatureLenBLSBLS12381)
	signature[0] = invalidBLSSignatureHeader // invalid header as per the Zcash serialization
	return signature
}

// decodePrivateKey decodes a slice of bytes into a private key.
// Decoding assumes a bytes big endian format.
// It checks the scalar is non-zero and is less than the group order.
func (a *blsBLS12381Algo) decodePrivateKey(privateKeyBytes []byte) (PrivateKey, error) {
	sk := newPrKeyBLSBLS12381(nil)

	err := readScalarFrStar(&sk.scalar, privateKeyBytes)

	if err != nil {
		return nil, fmt.Errorf("failed to read the private key: %w", err)
	}
	return sk, nil
}

// decodePublicKey decodes a slice of bytes into a public key.
// This function includes a membership check in G2.
//
// Note the function does not reject the infinity point (identity element of G2).
// However, the comparison to identity is cached in the `PublicKey` structure for
// a faster check during signature verifications. Any verification against an identity
// public key outputs `false`.
func (a *blsBLS12381Algo) decodePublicKey(publicKeyBytes []byte) (PublicKey, error) {
	if len(publicKeyBytes) != PubKeyLenBLSBLS12381 {
		return nil, invalidInputsErrorf("input length must be %d, got %d",
			PubKeyLenBLSBLS12381, len(publicKeyBytes))
	}
	var pk pubKeyBLSBLS12381
	err := readPointE2(&pk.point, publicKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("decode public key failed: %w", err)
	}

	// membership check in G2
	if !bool(C.E2_in_G2((*C.E2)(&pk.point))) {
		return nil, invalidInputsErrorf("input key is infinity or does not encode a BLS12-381 point in the valid group")
	}

	// check point is non-infinity and cache it
	pk.isIdentity = (&pk.point).isInfinity()

	return &pk, nil
}

// decodePublicKeyCompressed decodes a slice of bytes into a public key.
// since we use the compressed representation by default, this checks the default and delegates to decodePublicKeyCompressed
func (a *blsBLS12381Algo) decodePublicKeyCompressed(publicKeyBytes []byte) (PublicKey, error) {
	if !isG2Compressed() {
		panic("library is not configured to use compressed public key serialization")
	}
	return a.decodePublicKey(publicKeyBytes)
}

// prKeyBLSBLS12381 is the private key of BLS using BLS12_381, it implements PrivateKey

var _ PrivateKey = (*prKeyBLSBLS12381)(nil)

type prKeyBLSBLS12381 struct {
	// public key
	pk *pubKeyBLSBLS12381
	// private key data
	scalar scalar
}

// newPrKeyBLSBLS12381 creates a new BLS private key with the given scalar.
// If no scalar is provided, the function allocates an
// empty scalar.
func newPrKeyBLSBLS12381(x *scalar) *prKeyBLSBLS12381 {
	if x != nil {
		return &prKeyBLSBLS12381{
			// the embedded public key is only computed when needed
			scalar: *x,
		}
	}
	return &prKeyBLSBLS12381{}
}

// Algorithm returns the Signing Algorithm
func (sk *prKeyBLSBLS12381) Algorithm() SigningAlgorithm {
	return BLSBLS12381
}

// Size returns the private key length in bytes
func (sk *prKeyBLSBLS12381) Size() int {
	return PrKeyLenBLSBLS12381
}

// computePublicKey generates the public key corresponding to
// the input private key. The function makes sure the public key
// is valid in G2.
func (sk *prKeyBLSBLS12381) computePublicKey() {
	var newPk pubKeyBLSBLS12381
	// compute public key pk = g2^sk
	generatorScalarMultG2(&newPk.point, &sk.scalar)

	// cache the identity comparison
	newPk.isIdentity = (&sk.scalar).isZero()

	sk.pk = &newPk
}

// PublicKey returns the public key corresponding to the private key
func (sk *prKeyBLSBLS12381) PublicKey() PublicKey {
	if sk.pk != nil {
		return sk.pk
	}
	sk.computePublicKey()
	return sk.pk
}

// Encode returns a byte encoding of the private key.
// The encoding is a raw encoding in big endian padded to the group order
func (a *prKeyBLSBLS12381) Encode() []byte {
	dest := make([]byte, frBytesLen)
	writeScalar(dest, &a.scalar)
	return dest
}

// Equals checks is two public keys are equal.
func (sk *prKeyBLSBLS12381) Equals(other PrivateKey) bool {
	otherBLS, ok := other.(*prKeyBLSBLS12381)
	if !ok {
		return false
	}
	return (&sk.scalar).equals(&otherBLS.scalar)
}

// String returns the hex string representation of the key.
func (sk *prKeyBLSBLS12381) String() string {
	return sk.scalar.String()
}

// pubKeyBLSBLS12381 is the public key of BLS using BLS12_381,
// it implements PublicKey.

var _ PublicKey = (*pubKeyBLSBLS12381)(nil)

type pubKeyBLSBLS12381 struct {
	// The package guarantees an instance is only created with a point
	// on the correct G2 subgroup. No membership check is needed when the
	// instance is used in any BLS function.
	// However, an instance can be created with an infinity point. Although
	// infinity is a valid G2 point, some BLS functions fail (return false)
	// when used with an infinity point. The package caches the infinity
	// comparison in pubKeyBLSBLS12381 for a faster check. The package makes
	// sure the comparison is performed after an instance is created.
	//
	// public key G2 point
	point pointE2
	// G2 identity check cache
	isIdentity bool
}

// newPubKeyBLSBLS12381 creates a new BLS public key with the given point.
// If no scalar is provided, the function allocates an
// empty scalar.
func newPubKeyBLSBLS12381(p *pointE2) *pubKeyBLSBLS12381 {
	if p != nil {
		key := &pubKeyBLSBLS12381{
			point: *p,
		}
		// cache the identity comparison for a faster check
		// during signature verifications
		key.isIdentity = p.isInfinity()
		return key
	}
	return &pubKeyBLSBLS12381{}
}

// Algorithm returns the Signing Algorithm
func (pk *pubKeyBLSBLS12381) Algorithm() SigningAlgorithm {
	return BLSBLS12381
}

// Size returns the public key lengh in bytes
func (pk *pubKeyBLSBLS12381) Size() int {
	return PubKeyLenBLSBLS12381
}

// EncodeCompressed returns a byte encoding of the public key.
// The encoding is a compressed encoding of the point
// [zcash] https://www.ietf.org/archive/id/draft-irtf-cfrg-pairing-friendly-curves-08.html#name-zcash-serialization-format-
func (a *pubKeyBLSBLS12381) EncodeCompressed() []byte {
	if !isG2Compressed() {
		panic("library is not configured to use compressed public key serialization")
	}
	return a.Encode()
}

// Encode returns a byte encoding of the public key (a G2 point).
// The current encoding is a compressed serialization of G2 following [zcash] https://www.ietf.org/archive/id/draft-irtf-cfrg-pairing-friendly-curves-08.html#name-zcash-serialization-format-
//
// The function should evolve in the future to support uncompressed compresion too.
func (a *pubKeyBLSBLS12381) Encode() []byte {
	dest := make([]byte, g2BytesLen)
	writePointE2(dest, &a.point)
	return dest
}

// Equals checks is two public keys are equal
func (pk *pubKeyBLSBLS12381) Equals(other PublicKey) bool {
	otherBLS, ok := other.(*pubKeyBLSBLS12381)
	if !ok {
		return false
	}
	return pk.point.equals(&otherBLS.point)
}

// String returns the hex string representation of the key.
func (pk *pubKeyBLSBLS12381) String() string {
	return pk.point.String()
}

// This is only a TEST function.
// signWithXMDSHA256 signs a message using XMD_SHA256 as a hash to field.
//
// The function is in this file because cgo can't be used in go test files.
// TODO: implement a hasher for XMD SHA256 and use the `Sign` function.
func (sk *prKeyBLSBLS12381) signWithXMDSHA256(data []byte) Signature {

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
		(*C.Fr)(&sk.scalar),
		(*C.uchar)(&hash[0]),
		(C.int)(len(hash)))
	return s
}
