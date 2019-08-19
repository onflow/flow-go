package crypto

import (
	goecdsa "crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"math/big"

	log "github.com/sirupsen/logrus"
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
func (a *ECDSAalgo) SignHash(sk PrKey, h Hash) Signature {
	skECDSA, ok := sk.(*PrKeyECDSA)
	if !ok {
		log.Error("ECDSA sigature can only be called using an ECDSA private key")
		return nil
	}
	r, s, err := goecdsa.Sign(rand.Reader, skECDSA.goPrKey, h.Bytes())
	if err != nil {
		log.Error("Signature has failed")
		return nil
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
	return signature
}

// SignBytes signs an array of bytes
func (a *ECDSAalgo) SignBytes(sk PrKey, data []byte, alg Hasher) Signature {
	if alg == nil {
		log.Error("SignBytes requires a Hasher")
		return nil
	}
	h := alg.ComputeBytesHash(data)
	return a.SignHash(sk, h)
}

// SignStruct signs a structure
func (a *ECDSAalgo) SignStruct(sk PrKey, data Encoder, alg Hasher) Signature {
	if alg == nil {
		log.Error("SignStruct requires a Hasher")
		return nil
	}
	h := alg.ComputeStructHash(data)
	return a.SignHash(sk, h)
}

// VerifyHash implements ECDSA signature verification
func (a *ECDSAalgo) VerifyHash(pk PubKey, sig Signature, h Hash) bool {
	ecdsaPk, ok := pk.(*PubKeyECDSA)
	if !ok {
		log.Error("ECDSA signature verification can only be called using an ECDSA public key")
		return false
	}
	var r big.Int
	var s big.Int
	Nlen := bitToBytes((a.curve.Params().N).BitLen())
	r.SetBytes(sig[:Nlen])
	s.SetBytes(sig[Nlen:])
	return goecdsa.Verify((*goecdsa.PublicKey)(ecdsaPk), h.Bytes(), &r, &s)
}

// VerifyBytes verifies a signature of a byte array
func (a *ECDSAalgo) VerifyBytes(pk PubKey, sig Signature, data []byte, alg Hasher) bool {
	if alg == nil {
		log.Error("VerifyBytes requires a Hasher")
		return false
	}
	h := alg.ComputeBytesHash(data)
	return a.VerifyHash(pk, sig, h)
}

// VerifyStruct verifies a signature of a structure
func (a *ECDSAalgo) VerifyStruct(pk PubKey, sig Signature, data Encoder, alg Hasher) bool {
	if alg == nil {
		log.Error("VerifyStruct requires a Hasher")
		return false
	}
	h := alg.ComputeStructHash(data)
	return a.VerifyHash(pk, sig, h)
}

// GeneratePrKey generates a private key for ECDSA
// This is only a test function!
func (a *ECDSAalgo) GeneratePrKey(seed []byte) PrKey {
	sk, err := goecdsa.GenerateKey(a.curve, rand.Reader)
	if err != nil {
		log.Error("The ECDSA key generation has failed")
		return nil
	}
	return &(PrKeyECDSA{a, sk})
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
