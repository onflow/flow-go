package crypto

import (
	goecdsa "crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"

	//"crypto/rand"

	log "github.com/sirupsen/logrus"
)

// ECDSAalgo embeds SignAlgo
type ECDSAalgo struct {
	// elliptic curve
	curve elliptic.Curve
	// embeds SignAlgo
	*SignAlgo
}

// SignHash implements ECDSA signature
func (a *ECDSAalgo) SignHash(sk PrKey, h Hash) Signature {
	skECDSA, ok := sk.(*PrKeyECDSA)
	if ok == false {
		log.Error("ECDSA sigature can only be called using an ECDSA private key")
		return nil
	}
	r, s, err := goecdsa.Sign(rand.Reader, skECDSA.goPrKey, h.Bytes())
	if err != nil {
		log.Error("Signature has failed")
	}
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	Nlen := ((a.curve.Params().N).BitLen() + 7) / 8
	signature := make([]byte, 2*Nlen)
	// padd the signature with zeroes
	i := 0
	for ; i < Nlen-len(rBytes); i++ {
		signature[i] = 0
	}
	copy(signature[i:], rBytes)
	for ; i < 2*Nlen-len(sBytes); i++ {
		signature[i] = 0
	}
	copy(signature[i:], rBytes)
	return signature
}

// SignBytes signs an array of bytes
func (a *ECDSAalgo) SignBytes(sk PrKey, data []byte, alg Hasher) Signature {
	if alg == nil {
		log.Error("SignStruct requires a Hasher")
	}
	h := alg.ComputeBytesHash(data)
	return a.SignHash(sk, h)
}

// SignStruct signs a structure
func (a *ECDSAalgo) SignStruct(sk PrKey, data Encoder, alg Hasher) Signature {
	if alg == nil {
		log.Error("SignStruct requires a Hasher")
	}
	h := alg.ComputeStructHash(data)
	return a.SignHash(sk, h)
}

// VerifyHash implements ECDSA signature verification
func (a *ECDSAalgo) VerifyHash(pk PubKey, s Signature, h Hash) bool {
	return true
}

// VerifyBytes verifies a signature of a byte array
func (a *ECDSAalgo) VerifyBytes(pk PubKey, s Signature, data []byte, alg Hasher) bool {
	return true
}

// VerifyStruct verifies a signature of a structure
func (a *ECDSAalgo) VerifyStruct(pk PubKey, s Signature, data Encoder, alg Hasher) bool {
	return true
}

// GeneratePrKey generates a private key for ECDSA
// This is only a test function!
func (a *ECDSAalgo) GeneratePrKey(seed []byte) PrKey {
	sk, err := goecdsa.GenerateKey(a.curve, rand.Reader)
	if err != nil {
		log.Error("The ECDSA key generation has failed")
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
	return ((sk.alg.curve.Params().N).BitLen() + 7) / 8
}

// Pubkey returns the public key associated to the private key
func (sk *PrKeyECDSA) Pubkey() PubKey {
	pk, ok := (sk.goPrKey.Public()).(*PubKeyECDSA)
	if ok == false {
		log.Error("Please check the definition of PubKeyECDSA")
		return nil
	}
	return pk
}

// PubKeyECDSA is the public key of ECDSA, it implements PubKey
type PubKeyECDSA goecdsa.PublicKey

// KeySize returns the length of the public key
func (pk *PubKeyECDSA) KeySize() int {
	return 2 * (((pk.Params().P).BitLen() + 7) / 8)
}
