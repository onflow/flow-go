package crypto

// Elliptic Curve Digital Signature Algorithm is implemented as
// defined in FIPS 186-4, although the hash function implemented in this package is SHA3.
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
	// embeds SignAlgo
	*SignAlgo
}

func bitToBytes(bits int) int {
	return (bits + 7) >> 3
}

// SignHash implements ECDSA signature
func (a *ECDSAalgo) SignHash(sk PrKey, h Hash) (Signature, error) {
	skECDSA, ok := sk.(*PrKeyECDSA)
	if !ok {
		return nil, cryptoError{"ECDSA sigature can only be called using an ECDSA private key"}
	}
	r, s, err := goecdsa.Sign(rand.Reader, skECDSA.goPrKey, h.Bytes())
	if err != nil {
		return nil, cryptoError{"ECDSA Signature has failed"}
	}
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	Nlen := bitToBytes((a.curve.Params().N).BitLen())
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

// SignBytes signs an array of bytes
func (a *ECDSAalgo) SignBytes(sk PrKey, data []byte, alg Hasher) (Signature, error) {
	if alg == nil {
		return nil, cryptoError{"ECDSA SignBytes requires a Hasher"}
	}
	h := alg.ComputeBytesHash(data)
	return a.SignHash(sk, h)
}

// SignStruct signs a structure
func (a *ECDSAalgo) SignStruct(sk PrKey, data Encoder, alg Hasher) (Signature, error) {
	if alg == nil {
		return nil, cryptoError{"ECDSA SignStruct requires a Hasher"}
	}
	h := alg.ComputeStructHash(data)
	return a.SignHash(sk, h)
}

// VerifyHash implements ECDSA signature verification
func (a *ECDSAalgo) VerifyHash(pk PubKey, sig Signature, h Hash) (bool, error) {
	ecdsaPk, ok := pk.(*PubKeyECDSA)
	if !ok {
		return false, cryptoError{"ECDSA signature verification can only be called using an ECDSA public key"}
	}
	var r big.Int
	var s big.Int
	Nlen := bitToBytes((a.curve.Params().N).BitLen())
	r.SetBytes(sig[:Nlen])
	s.SetBytes(sig[Nlen:])
	return goecdsa.Verify((*goecdsa.PublicKey)(ecdsaPk), h.Bytes(), &r, &s), nil
}

// VerifyBytes verifies a signature of a byte array
func (a *ECDSAalgo) VerifyBytes(pk PubKey, sig Signature, data []byte, alg Hasher) (bool, error) {
	if alg == nil {
		return false, cryptoError{"VerifyBytes requires a Hasher"}
	}
	h := alg.ComputeBytesHash(data)
	return a.VerifyHash(pk, sig, h)
}

// VerifyStruct verifies a signature of a structure
func (a *ECDSAalgo) VerifyStruct(pk PubKey, sig Signature, data Encoder, alg Hasher) (bool, error) {
	if alg == nil {
		return false, cryptoError{"VerifyStruct requires a Hasher"}
	}
	h := alg.ComputeStructHash(data)
	return a.VerifyHash(pk, sig, h)
}

// GeneratePrKey generates a private key for ECDSA
// This is only a test function!
func (a *ECDSAalgo) GeneratePrKey(seed []byte) (PrKey, error) {
	sk, err := goecdsa.GenerateKey(a.curve, rand.Reader)
	if err != nil {
		return nil, cryptoError{"The ECDSA key generation has failed"}
	}
	return &(PrKeyECDSA{a, sk}), nil
}

func (a *ECDSAalgo) EncodePrKey(sk PrKey) ([]byte, error) {
	skECDSA, ok := sk.(*PrKeyECDSA)
	if !ok {
		return nil, cryptoError{"key is not an ECDSA private key"}
	}

	der, err := x509.MarshalECPrivateKey(skECDSA.goPrKey)
	if err != nil {
		return nil, err
	}

	return der, nil
}

func (a *ECDSAalgo) ParsePrKey(der []byte) (PrKey, error) {
	sk, err := x509.ParseECPrivateKey(der)
	if err != nil {
		return nil, err
	}

	return &(PrKeyECDSA{a, sk}), nil
}

func (a *ECDSAalgo) EncodePubKey(pk PubKey) ([]byte, error) {
	ecdsaPk, ok := pk.(*PubKeyECDSA)
	if !ok {
		return nil, cryptoError{"key is not an ECDSA public key"}
	}

	b := elliptic.Marshal(a.curve, ecdsaPk.X, ecdsaPk.Y)
	return b, nil
}

func (a *ECDSAalgo) ParsePubKey(b []byte) (PubKey, error) {
	x, y := elliptic.Unmarshal(a.curve, b)
	return &PubKeyECDSA{a.curve, x, y}, nil
}

// PrKeyECDSA is the private key of ECDSA, it implements PrKey
type PrKeyECDSA struct {
	// the signature algo
	alg *ECDSAalgo
	// private key (including the public key)
	goPrKey *goecdsa.PrivateKey
}

// AlgoName returns the name of the algo related to the private key
func (sk *PrKeyECDSA) AlgoName() AlgoName {
	return sk.alg.Name()
}

// KeySize returns the length of the private key
func (sk *PrKeyECDSA) KeySize() int {
	return bitToBytes((sk.alg.curve.Params().N).BitLen())
}

// Pubkey returns the public key associated to the private key
func (sk *PrKeyECDSA) Pubkey() PubKey {
	pk := sk.goPrKey.PublicKey
	return (*PubKeyECDSA)(&pk)
}

// PubKeyECDSA is the public key of ECDSA, it implements PubKey
type PubKeyECDSA goecdsa.PublicKey

// KeySize returns the length of the public key
func (pk *PubKeyECDSA) KeySize() int {
	return 2 * bitToBytes((pk.Params().P).BitLen())
}
