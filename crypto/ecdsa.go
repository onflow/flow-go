package crypto

// Elliptic Curve Digital Signature Algorithm is implemented as
// defined in FIPS 186-4, although The hash function implemented in this package is SHA3.
// This is different from the ECDSA version implemented in some blockchains.

import (
	"bytes"
	goecdsa "crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"fmt"
	"math/big"
)

// ECDSAalgo embeds SignAlgo
type ECDSAalgo struct {
	// elliptic curve
	curve elliptic.Curve
	// embeds commonSigner
	*commonSigner
}

func bitsToBytes(bits int) int {
	return (bits + 7) >> 3
}

// signHash implements ECDSA signature
func (sk *PrKeyECDSA) signHash(h Hash) (Signature, error) {
	r, s, err := goecdsa.Sign(rand.Reader, sk.goPrKey, h)
	if err != nil {
		return nil, cryptoError{"ECDSA Signature has failed"}
	}
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	Nlen := bitsToBytes((sk.alg.curve.Params().N).BitLen())
	signature := make([]byte, 2*Nlen)
	// pad the signature with zeroes
	i := 0
	for ; i < Nlen-len(rBytes); i++ {
		signature[i] = 0
	}
	i += copy(signature[i:], rBytes)
	for ; i < 2*Nlen-len(sBytes); i++ {
		signature[i] = 0
	}
	copy(signature[i:], sBytes)
	return signature, nil
}

// Sign signs an array of bytes
func (sk *PrKeyECDSA) Sign(data []byte, alg Hasher) (Signature, error) {
	h := alg.ComputeHash(data)
	return sk.signHash(h)
}

// verifyHash implements ECDSA signature verification
func (pk *PubKeyECDSA) verifyHash(sig Signature, h Hash) (bool, error) {
	var r big.Int
	var s big.Int
	Nlen := bitsToBytes((pk.alg.curve.Params().N).BitLen())
	r.SetBytes(sig[:Nlen])
	s.SetBytes(sig[Nlen:])
	return goecdsa.Verify(pk.goPubKey, h, &r, &s), nil
}

// Verify verifies a signature of a byte array
func (pk *PubKeyECDSA) Verify(sig Signature, data []byte, alg Hasher) (bool, error) {
	if alg == nil {
		return false, cryptoError{"VerifyBytes requires a Hasher"}
	}
	h := alg.ComputeHash(data)
	return pk.verifyHash(sig, h)
}

// GeneratePrKey generates a private key for ECDSA
// This is only a test function!
func (a *ECDSAalgo) generatePrivateKey(seed []byte) (PrivateKey, error) {
	Nlen := bitsToBytes((a.curve.Params().N).BitLen())
	if len(seed) < Nlen+8 {
		return nil, cryptoError{fmt.Sprintf("the seed should be at least %d bytes.", Nlen+8)}
	}
	sk, err := goecdsa.GenerateKey(a.curve, bytes.NewReader(seed))
	if err != nil {
		return nil, cryptoError{"The ECDSA key generation has failed"}
	}
	return &PrKeyECDSA{a, sk}, nil
}

func (a *ECDSAalgo) rawDecodePrivateKey(der []byte) (PrivateKey, error) {
	Nlen := bitsToBytes((a.curve.Params().N).BitLen())
	if len(der) != Nlen {
		return nil, cryptoError{"the raw private key is not valid."}
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

func (a *ECDSAalgo) decodePrivateKey(der []byte) (PrivateKey, error) {
	// NIST curves case
	if a.algo == ECDSA_P256 {
		sk, err := x509.ParseECPrivateKey(der)
		if err != nil {
			return nil, err
		}
		return &PrKeyECDSA{a, sk}, nil
	}
	// non-NIST curves
	if a.algo == ECDSA_SECp256k1 {
		return a.rawDecodePrivateKey(der)
	}
	return nil, cryptoError{"The curve is not supported."}
}

func (a *ECDSAalgo) rawDecodePublicKey(der []byte) (PublicKey, error) {
	Plen := bitsToBytes((a.curve.Params().N).BitLen())
	if len(der) != 2*Plen {
		return nil, cryptoError{"the raw public key is not valid."}
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

func (a *ECDSAalgo) decodePublicKey(der []byte) (PublicKey, error) {
	// NIST curves case
	if a.algo == ECDSA_P256 {
		i, err := x509.ParsePKIXPublicKey(der)
		if err != nil {
			return nil, err
		}
		goecdsaPk := i.(*goecdsa.PublicKey)
		return &PubKeyECDSA{
			alg:      a,
			goPubKey: goecdsaPk,
		}, nil
	}
	// non-NIST curves
	if a.algo == ECDSA_SECp256k1 {
		return a.rawDecodePublicKey(der)
	}
	return nil, cryptoError{"The curve is not supported."}
}

// PrKeyECDSA is the private key of ECDSA, it implements PrivateKey
type PrKeyECDSA struct {
	// the signature algo
	alg *ECDSAalgo
	// private key (including the public key)
	goPrKey *goecdsa.PrivateKey
}

// Algorithm returns the name of the algo related to the private key
func (sk *PrKeyECDSA) Algorithm() SigningAlgorithm {
	return sk.alg.algo
}

// KeySize returns the length of the private key
func (sk *PrKeyECDSA) KeySize() int {
	return bitsToBytes((sk.alg.curve.Params().N).BitLen())
}

// PublicKey returns the public key associated to the private key
func (sk *PrKeyECDSA) PublicKey() PublicKey {
	return &PubKeyECDSA{
		alg:      sk.alg,
		goPubKey: &sk.goPrKey.PublicKey,
	}
}

func (sk *PrKeyECDSA) rawEncode() ([]byte, error) {
	skBytes := sk.goPrKey.D.Bytes()
	Nlen := bitsToBytes((sk.alg.curve.Params().N).BitLen())
	skEncoded := make([]byte, Nlen)
	// pad sk with zeroes
	i := 0
	for ; i < Nlen-len(skBytes); i++ {
		skEncoded[i] = 0
	}
	copy(skEncoded[i:], skBytes)
	return skEncoded, nil
}

func (sk *PrKeyECDSA) Encode() ([]byte, error) {
	// NIST curves case
	if sk.Algorithm() == ECDSA_P256 {
		return x509.MarshalECPrivateKey(sk.goPrKey)
	}
	// non-NIST curves
	if sk.Algorithm() == ECDSA_SECp256k1 {
		return sk.rawEncode()
	}
	return nil, cryptoError{"The curve is not supported."}
}

// PubKeyECDSA is the public key of ECDSA, it implements PublicKey
type PubKeyECDSA struct {
	// the signature algo
	alg *ECDSAalgo
	// public key data
	goPubKey *goecdsa.PublicKey
}

// AlgoName returns the name of the algo related to the private key
func (pk *PubKeyECDSA) Algorithm() SigningAlgorithm {
	return pk.alg.algo
}

// KeySize returns the length of the public key
func (pk *PubKeyECDSA) KeySize() int {
	return 2 * bitsToBytes((pk.goPubKey.Params().P).BitLen())
}

func (pk *PubKeyECDSA) rawEncode() ([]byte, error) {
	xBytes := pk.goPubKey.X.Bytes()
	yBytes := pk.goPubKey.Y.Bytes()
	Plen := bitsToBytes((pk.alg.curve.Params().P).BitLen())
	pkEncoded := make([]byte, 2*Plen)
	// pad the public key coordinates with zeroes
	i := 0
	for ; i < Plen-len(xBytes); i++ {
		pkEncoded[i] = 0
	}
	i += copy(pkEncoded[i:], xBytes)
	for ; i < 2*Plen-len(yBytes); i++ {
		pkEncoded[i] = 0
	}
	copy(pkEncoded[i:], yBytes)
	return pkEncoded, nil
}

func (pk *PubKeyECDSA) Encode() ([]byte, error) {
	// NIST curves case
	if pk.Algorithm() == ECDSA_P256 {
		return x509.MarshalPKIXPublicKey(pk.goPubKey)
	}
	// non-NIST curves
	if pk.Algorithm() == ECDSA_SECp256k1 {
		return pk.rawEncode()
	}
	return nil, cryptoError{"The curve is not supported."}
}
