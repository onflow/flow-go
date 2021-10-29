package crypto

// Elliptic Curve Digital Signature Algorithm is implemented as
// defined in FIPS 186-4 (although the hash functions implemented in this package are SHA2 and SHA3).

// Most of the implementation is Go based and is not optimized for performance.

// This implementation does not include any security against side-channel attacks.

import (
	goecdsa "crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/btcsuite/btcd/btcec"
	"github.com/onflow/flow-go/crypto/hash"
)

// ecdsaAlgo embeds SignAlgo
type ecdsaAlgo struct {
	// elliptic curve
	curve elliptic.Curve
	// the signing algo and parameters
	algo SigningAlgorithm
}

//  ECDSA contexts for each supported curve
// NIST P-256 curve
var p256Instance *ecdsaAlgo

// SECG secp256k1 curve https://www.secg.org/sec2-v2.pdf
var secp256k1Instance *ecdsaAlgo

func bitsToBytes(bits int) int {
	return (bits + 7) >> 3
}

// signHash returns the signature of the hash using the private key
// the signature is the concatenation bytes(r)||bytes(s)
// where r and s are padded to the curve order size
func (sk *PrKeyECDSA) signHash(h hash.Hash) (Signature, error) {
	r, s, err := goecdsa.Sign(rand.Reader, sk.goPrKey, h)
	if err != nil {
		return nil, fmt.Errorf("ECDSA Sign failed: %w", err)
	}
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	Nlen := bitsToBytes((sk.alg.curve.Params().N).BitLen())
	signature := make([]byte, 2*Nlen)
	// pad the signature with zeroes
	copy(signature[Nlen-len(rBytes):], rBytes)
	copy(signature[2*Nlen-len(sBytes):], sBytes)
	return signature, nil
}

// Sign signs an array of bytes
//
// The private key is read only while sha2 and sha3 hashers are
// modified temporarily.
// The resulting signature is the concatenation bytes(r)||bytes(s),
// where r and s are padded to the curve order size.
func (sk *PrKeyECDSA) Sign(data []byte, alg hash.Hasher) (Signature, error) {
	// no need to check the hasher output size as all supported hash algos
	// have at least 32 bytes output
	if alg == nil {
		return nil, invalidInputsErrorf("Sign requires a Hasher")
	}
	h := alg.ComputeHash(data)
	return sk.signHash(h)
}

// verifyHash implements ECDSA signature verification
func (pk *PubKeyECDSA) verifyHash(sig Signature, h hash.Hash) (bool, error) {
	Nlen := bitsToBytes((pk.alg.curve.Params().N).BitLen())

	if len(sig) != 2*Nlen {
		return false, nil
	}

	var r big.Int
	var s big.Int
	r.SetBytes(sig[:Nlen])
	s.SetBytes(sig[Nlen:])
	return goecdsa.Verify(pk.goPubKey, h, &r, &s), nil
}

// Verify verifies a signature of an input data under the public key.
//
// If the input signature slice has an invalid length or fails to deserialize into valid
// scalars, the function returns false without an error.
//
// Public keys are read only, sha2 and sha3 hashers are
// modified temporarily.
func (pk *PubKeyECDSA) Verify(sig Signature, data []byte, alg hash.Hasher) (bool, error) {
	// no need to check the hasher output size as all supported hash algos
	// have at lease 32 bytes output
	if alg == nil {
		return false, invalidInputsErrorf("Verify requires a Hasher")
	}

	h := alg.ComputeHash(data)
	return pk.verifyHash(sig, h)
}

// signatureFormatCheck verifies the format of a serialized signature,
// regardless of messages or public keys.
// If FormatCheck returns false then the input is not a valid ECDSA
// signature and will fail a verification against any message and public key.
func (a *ecdsaAlgo) signatureFormatCheck(sig Signature) bool {
	N := a.curve.Params().N
	Nlen := bitsToBytes(N.BitLen())

	if len(sig) != 2*Nlen {
		return false
	}

	var r big.Int
	var s big.Int
	r.SetBytes(sig[:Nlen])
	s.SetBytes(sig[Nlen:])

	if r.Sign() == 0 || s.Sign() == 0 {
		return false
	}

	if r.Cmp(N) >= 0 || s.Cmp(N) >= 0 {
		return false
	}

	// We could also check whether r and r+N are quadratic residues modulo (p)
	// using Euler's criterion.
	return true
}

var one = new(big.Int).SetInt64(1)

// goecdsaGenerateKey generates a public and private key pair
// for the crypto/ecdsa library using the input seed as input
func goecdsaGenerateKey(c elliptic.Curve, seed []byte) *goecdsa.PrivateKey {
	k := new(big.Int).SetBytes(seed)
	n := new(big.Int).Sub(c.Params().N, one)
	k.Mod(k, n)
	k.Add(k, one)

	priv := new(goecdsa.PrivateKey)
	priv.PublicKey.Curve = c
	priv.D = k
	// public key is not computed
	return priv
}

// generatePrivateKey generates a private key for ECDSA
// deterministically using the input seed.
//
// It is recommended to use a secure crypto RNG to generate the seed.
// The seed must have enough entropy and should be sampled uniformly at random.
func (a *ecdsaAlgo) generatePrivateKey(seed []byte) (PrivateKey, error) {
	Nlen := bitsToBytes((a.curve.Params().N).BitLen())
	// use extra 128 bits to reduce the modular reduction bias
	minSeedLen := Nlen + (securityBits / 8)
	if len(seed) < minSeedLen || len(seed) > KeyGenSeedMaxLenECDSA {
		return nil, invalidInputsErrorf("seed byte length should be between %d and %d",
			minSeedLen, KeyGenSeedMaxLenECDSA)
	}
	sk := goecdsaGenerateKey(a.curve, seed)
	return &PrKeyECDSA{
		alg:     a,
		goPrKey: sk,
		pubKey:  nil, // public key is not computed
	}, nil
}

func (a *ecdsaAlgo) rawDecodePrivateKey(der []byte) (PrivateKey, error) {
	n := a.curve.Params().N
	nlen := bitsToBytes(n.BitLen())
	if len(der) != nlen {
		return nil, invalidInputsErrorf("input has incorrect %s key size", a.algo)
	}
	var d big.Int
	d.SetBytes(der)

	if d.Cmp(n) >= 0 {
		return nil, invalidInputsErrorf("input is not a valid %s key", a.algo)
	}

	priv := goecdsa.PrivateKey{
		D: &d,
	}
	priv.PublicKey.Curve = a.curve

	return &PrKeyECDSA{
		alg:     a,
		goPrKey: &priv,
		pubKey:  nil, // public key is not computed
	}, nil
}

func (a *ecdsaAlgo) decodePrivateKey(der []byte) (PrivateKey, error) {
	return a.rawDecodePrivateKey(der)
}

func (a *ecdsaAlgo) rawDecodePublicKey(der []byte) (PublicKey, error) {
	p := (a.curve.Params().P)
	plen := bitsToBytes(p.BitLen())
	if len(der) != 2*plen {
		return nil, invalidInputsErrorf("input has incorrect %s key size", a.algo)
	}
	var x, y big.Int
	x.SetBytes(der[:plen])
	y.SetBytes(der[plen:])

	// all the curves supported for now have a cofactor equal to 1,
	// so that IsOnCurve guarantees the point is on the right subgroup.
	if x.Cmp(p) >= 0 || y.Cmp(p) >= 0 || !a.curve.IsOnCurve(&x, &y) {
		return nil, invalidInputsErrorf("input %x is not a valid %s key", der, a.algo)
	}

	pk := goecdsa.PublicKey{
		Curve: a.curve,
		X:     &x,
		Y:     &y,
	}

	return &PubKeyECDSA{a, &pk}, nil
}

func (a *ecdsaAlgo) decodePublicKey(der []byte) (PublicKey, error) {
	return a.rawDecodePublicKey(der)
}

// decodePublicKeyCompressed returns a public key given the bytes of a compressed public key according to X9.62 section 4.3.6.
// this compressed representation uses an extra byte to disambiguate sign
func (a *ecdsaAlgo) decodePublicKeyCompressed(pkBytes []byte) (PublicKey, error) {
	expectedLen := bitsToBytes(a.curve.Params().BitSize) + 1
	if len(pkBytes) != expectedLen {
		return nil, invalidInputsErrorf(fmt.Sprintf("input length incompatible, expected %d, got %d", expectedLen, len(pkBytes)))
	}
	var goPubKey *goecdsa.PublicKey

	if a.curve == elliptic.P256() {
		x, y := elliptic.UnmarshalCompressed(a.curve, pkBytes)
		if x == nil {
			return nil, invalidInputsErrorf("Key %x can't be interpreted as %v", pkBytes, a.algo.String())
		}
		goPubKey = new(goecdsa.PublicKey)
		goPubKey.Curve = a.curve
		goPubKey.X = x
		goPubKey.Y = y

	} else if a.curve == btcec.S256() {
		pk, err := btcec.ParsePubKey(pkBytes, btcec.S256())
		if err != nil {
			return nil, invalidInputsErrorf("Key %x can't be interpreted as %v", pkBytes, a.algo.String())
		}
		// This assertion never fails
		goPubKey = (*goecdsa.PublicKey)(pk)
	} else {
		return nil, invalidInputsErrorf("the input curve is not supported")
	}
	return &PubKeyECDSA{a, goPubKey}, nil
}

// PrKeyECDSA is the private key of ECDSA, it implements the generic PrivateKey
type PrKeyECDSA struct {
	// the signature algo
	alg *ecdsaAlgo
	// goecdsa private key
	goPrKey *goecdsa.PrivateKey
	// public key
	pubKey *PubKeyECDSA
}

// Algorithm returns the algo related to the private key
func (sk *PrKeyECDSA) Algorithm() SigningAlgorithm {
	return sk.alg.algo
}

// Size returns the length of the private key in bytes
func (sk *PrKeyECDSA) Size() int {
	return bitsToBytes((sk.alg.curve.Params().N).BitLen())
}

// PublicKey returns the public key associated to the private key
func (sk *PrKeyECDSA) PublicKey() PublicKey {
	// compute the public key once
	if sk.pubKey == nil {
		priv := sk.goPrKey
		priv.PublicKey.X, priv.PublicKey.Y = priv.Curve.ScalarBaseMult(priv.D.Bytes())
	}
	sk.pubKey = &PubKeyECDSA{
		alg:      sk.alg,
		goPubKey: &sk.goPrKey.PublicKey,
	}
	return sk.pubKey
}

// given a private key (d), returns a raw encoding bytes(d) in big endian
// padded to the private key length
func (sk *PrKeyECDSA) rawEncode() []byte {
	skBytes := sk.goPrKey.D.Bytes()
	Nlen := bitsToBytes((sk.alg.curve.Params().N).BitLen())
	skEncoded := make([]byte, Nlen)
	// pad sk with zeroes
	copy(skEncoded[Nlen-len(skBytes):], skBytes)
	return skEncoded
}

// Encode returns a byte representation of a private key.
// a simple raw byte encoding in big endian is used for all curves
func (sk *PrKeyECDSA) Encode() []byte {
	return sk.rawEncode()
}

// Equals test the equality of two private keys
func (sk *PrKeyECDSA) Equals(other PrivateKey) bool {
	// check the key type
	otherECDSA, ok := other.(*PrKeyECDSA)
	if !ok {
		return false
	}
	// check the curve
	if sk.alg.curve != otherECDSA.alg.curve {
		return false
	}
	return sk.goPrKey.D.Cmp(otherECDSA.goPrKey.D) == 0
}

// String returns the hex string representation of the key.
func (sk *PrKeyECDSA) String() string {
	return fmt.Sprintf("%#x", sk.Encode())
}

// PubKeyECDSA is the public key of ECDSA, it implements PublicKey
type PubKeyECDSA struct {
	// the signature algo
	alg *ecdsaAlgo
	// public key data
	goPubKey *goecdsa.PublicKey
}

// Algorithm returns the the algo related to the private key
func (pk *PubKeyECDSA) Algorithm() SigningAlgorithm {
	return pk.alg.algo
}

// Size returns the length of the public key in bytes
func (pk *PubKeyECDSA) Size() int {
	return 2 * bitsToBytes((pk.goPubKey.Params().P).BitLen())
}

// EncodeCompressed returns a compressed encoding according to X9.62 section 4.3.6.
// This compressed representation uses an extra byte to disambiguate sign.
// The expected input is a public key (x,y).
func (pk *PubKeyECDSA) EncodeCompressed() []byte {
	if pk.alg.curve == btcec.S256() {
		return (*btcec.PublicKey)(pk.goPubKey).SerializeCompressed()
	}
	return elliptic.MarshalCompressed(pk.goPubKey, pk.goPubKey.X, pk.goPubKey.Y)
}

// given a public key (x,y), returns a raw uncompressed encoding bytes(x)||bytes(y)
// x and y are padded to the field size
func (pk *PubKeyECDSA) rawEncode() []byte {
	xBytes := pk.goPubKey.X.Bytes()
	yBytes := pk.goPubKey.Y.Bytes()
	Plen := bitsToBytes((pk.alg.curve.Params().P).BitLen())
	pkEncoded := make([]byte, 2*Plen)
	// pad the public key coordinates with zeroes
	copy(pkEncoded[Plen-len(xBytes):], xBytes)
	copy(pkEncoded[2*Plen-len(yBytes):], yBytes)
	return pkEncoded
}

// Encode returns a byte representation of a public key.
// a simple uncompressed raw encoding X||Y is used for all curves
// X and Y are the big endian byte encoding of the x and y coordinates of the public key
func (pk *PubKeyECDSA) Encode() []byte {
	return pk.rawEncode()
}

// Equals test the equality of two private keys
func (pk *PubKeyECDSA) Equals(other PublicKey) bool {
	// check the key type
	otherECDSA, ok := other.(*PubKeyECDSA)
	if !ok {
		return false
	}
	// check the curve
	if pk.alg.curve != otherECDSA.alg.curve {
		return false
	}
	return (pk.goPubKey.X.Cmp(otherECDSA.goPubKey.X) == 0) &&
		(pk.goPubKey.Y.Cmp(otherECDSA.goPubKey.Y) == 0)
}

// String returns the hex string representation of the key.
func (pk *PubKeyECDSA) String() string {
	return fmt.Sprintf("%#x", pk.Encode())
}
