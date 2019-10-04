package crypto

// Elliptic Curve Digital Signature Algorithm is implemented as
// defined in FIPS 186-4, although The hash function implemented in this package is SHA3.
// This is different from the ECDSA version implemented in some blockchains.

import (
	goecdsa "crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
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
	sk, err := goecdsa.GenerateKey(a.curve, rand.Reader)
	if err != nil {
		return nil, cryptoError{"The ECDSA key generation has failed"}
	}
	return &PrKeyECDSA{a, sk}, nil
}

func (a *ECDSAalgo) decodePrivateKey(der []byte) (PrivateKey, error) {
	sk, err := x509.ParseECPrivateKey(der)
	if err != nil {
		return nil, err
	}
	return &PrKeyECDSA{a, sk}, nil
}

func (a *ECDSAalgo) decodePublicKey(der []byte) (PublicKey, error) {
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

// PrKeyECDSA is the private key of ECDSA, it implements PrivateKey
type PrKeyECDSA struct {
	// the signature algo
	alg *ECDSAalgo
	// private key (including the public key)
	goPrKey *goecdsa.PrivateKey
}

// AlgoName returns the name of the algo related to the private key
func (sk *PrKeyECDSA) AlgoName() AlgoName {
	return sk.alg.name
}

// KeySize returns the length of the private key
func (sk *PrKeyECDSA) KeySize() int {
	return bitsToBytes((sk.alg.curve.Params().N).BitLen())
}

// Pubkey returns the public key associated to the private key
func (sk *PrKeyECDSA) Publickey() PublicKey {
	return &PubKeyECDSA{
		alg:      sk.alg,
		goPubKey: &sk.goPrKey.PublicKey,
	}
}

func (sk *PrKeyECDSA) Encode() ([]byte, error) {
	return x509.MarshalECPrivateKey(sk.goPrKey)
}

// PubKeyECDSA is the public key of ECDSA, it implements PublicKey
type PubKeyECDSA struct {
	// the signature algo
	alg *ECDSAalgo
	// public key data
	goPubKey *goecdsa.PublicKey
}

// AlgoName returns the name of the algo related to the private key
func (pk *PubKeyECDSA) AlgoName() AlgoName {
	return pk.alg.name
}

// KeySize returns the length of the public key
func (pk *PubKeyECDSA) KeySize() int {
	return 2 * bitsToBytes((pk.goPubKey.Params().P).BitLen())
}

func (pk *PubKeyECDSA) Encode() ([]byte, error) {
	return x509.MarshalPKIXPublicKey(pk.goPubKey)
}
