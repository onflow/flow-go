package crypto

// Elliptic Curve Digital Signature Algorithm is implemented as
// defined in FIPS 186-4, although The hash function implemented in this package is SHA3.
// This is different from the ECDSA version implemented in some blockchains.

import (
	goecdsa "crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/dapperlabs/flow-go/crypto/hash"
)

// ecdsaAlgo embeds SignAlgo
type ecdsaAlgo struct {
	// elliptic curve
	curve elliptic.Curve
	// the signing algo and parameters
	algo SigningAlgorithm
}

//  Once variables to use a unique instance
var p256Instance *ecdsaAlgo
var p256Once sync.Once

func newECDSAP256() *ecdsaAlgo {
	p256Once.Do(func() {
		p256Instance = &(ecdsaAlgo{
			curve: elliptic.P256(),
			algo:  ECDSAP256,
		})
	})
	return p256Instance
}

//  Once variables to use a unique instance
var secp256k1Instance *ecdsaAlgo
var secp256k1Once sync.Once

func newECDSASecp256k1() *ecdsaAlgo {
	secp256k1Once.Do(func() {
		secp256k1Instance = &(ecdsaAlgo{
			curve: secp256k1(),
			algo:  ECDSASecp256k1,
		})
	})
	return secp256k1Instance
}

func bitsToBytes(bits int) int {
	return (bits + 7) >> 3
}

// signHash returns the signature of the hash using the private key
// the signature is the concatenation bytes(r)||bytes(s)
// where r and s are padded to the curve order size
func (sk *PrKeyECDSA) signHash(h hash.Hash) (Signature, error) {
	r, s, err := goecdsa.Sign(rand.Reader, sk.goPrKey, h)
	if err != nil {
		return nil, fmt.Errorf("ECDSA Sign has failed: %w", err)
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
// It only reads the private key without modifiyong it while hashers sha2 and sha3 are
// modified temporarily.
// the resulting signature is the concatenation bytes(r)||bytes(s)
// where r and s are padded to the curve order size
func (sk *PrKeyECDSA) Sign(data []byte, alg hash.Hasher) (Signature, error) {
	if alg == nil {
		return nil, errors.New("Sign requires a Hasher")
	}
	h := alg.ComputeHash(data)
	return sk.signHash(h)
}

// verifyHash implements ECDSA signature verification
func (pk *PubKeyECDSA) verifyHash(sig Signature, h hash.Hash) (bool, error) {
	var r big.Int
	var s big.Int
	Nlen := bitsToBytes((pk.alg.curve.Params().N).BitLen())
	r.SetBytes(sig[:Nlen])
	s.SetBytes(sig[Nlen:])
	return goecdsa.Verify(pk.goPubKey, h, &r, &s), nil
}

// Verify verifies a signature of a byte array
// It only reads the public key. hashers sha2 and sha3 are
// modified temporarily
func (pk *PubKeyECDSA) Verify(sig Signature, data []byte, alg hash.Hasher) (bool, error) {
	if alg == nil {
		return false, errors.New("Verify requires a Hasher")
	}
	h := alg.ComputeHash(data)
	return pk.verifyHash(sig, h)
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
	priv.PublicKey.X, priv.PublicKey.Y = c.ScalarBaseMult(k.Bytes())
	return priv
}

// generatePrivateKey generates a private key for ECDSA
// deterministically using the input seed
func (a *ecdsaAlgo) generatePrivateKey(seed []byte) (PrivateKey, error) {
	Nlen := bitsToBytes((a.curve.Params().N).BitLen())
	// use extra 128 bits to reduce the modular reduction bias
	minSeedLen := Nlen + (securityBits / 8)
	if len(seed) < minSeedLen {
		return nil, fmt.Errorf("seed should be at least %d bytes", minSeedLen)
	}
	sk := goecdsaGenerateKey(a.curve, seed)
	return &PrKeyECDSA{a, sk}, nil
}

func (a *ecdsaAlgo) rawDecodePrivateKey(der []byte) (PrivateKey, error) {
	Nlen := bitsToBytes((a.curve.Params().N).BitLen())
	if len(der) != Nlen {
		return nil, errors.New("raw private key is not valid")
	}
	var d big.Int
	d.SetBytes(der)

	priv := goecdsa.PrivateKey{
		D: &d,
	}
	priv.PublicKey.Curve = a.curve
	priv.PublicKey.X, priv.PublicKey.Y = a.curve.ScalarBaseMult(der)
	return &PrKeyECDSA{a, &priv}, nil
}

func (a *ecdsaAlgo) decodePrivateKey(der []byte) (PrivateKey, error) {
	return a.rawDecodePrivateKey(der)
}

func (a *ecdsaAlgo) rawDecodePublicKey(der []byte) (PublicKey, error) {
	Plen := bitsToBytes((a.curve.Params().P).BitLen())
	if len(der) != 2*Plen {
		return nil, errors.New("raw public key is not valid")
	}
	var x, y big.Int
	x.SetBytes(der[:Plen])
	y.SetBytes(der[Plen:])

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

// PrKeyECDSA is the private key of ECDSA, it implements the generic PrivateKey
type PrKeyECDSA struct {
	// the signature algo
	alg *ecdsaAlgo
	// private key (including the public key)
	goPrKey *goecdsa.PrivateKey
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
	return &PubKeyECDSA{
		alg:      sk.alg,
		goPubKey: &sk.goPrKey.PublicKey,
	}
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
// X and Y are the big endian byte encoding of the x and y coordinate of the public key
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
