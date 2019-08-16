package crypto

import (
	goecdsa "crypto/ecdsa"
	"crypto/elliptic"
)

// ECDSAalgo, embeds SignAlgo
type ECDSAalgo struct {
	// elliptic curve
	elliptic.CurveParams
	// embeds SignAlgo
	*SignAlgo
}

// SignHash implements ECDSA signature
func (a *ECDSAalgo) SignHash(sk PrKey, h Hash) Signature {
	return []byte{0}
}

// SignBytes signs an array of bytes
func (a *ECDSAalgo) SignBytes(sk PrKey, data []byte, alg Hasher) Signature {
	return []byte{0}
}

// SignStruct signs a structure
func (a *ECDSAalgo) SignStruct(sk PrKey, data Encoder, alg Hasher) Signature {
	return []byte{0}
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

// GeneratePrKey generates a private key for BLS on BLS12381 curve
func (a *ECDSAalgo) GeneratePrKey(seed []byte) PrKey {
	var sk PrKeyBLS_BLS12381
	// Generate private key here
	randZr(&(sk.scalar), seed)
	// public key is not computed (but this could be changed)
	sk.pk = nil
	// links the private key to the algo
	sk.alg = a
	return &sk
}

// PrKeyBLS_BLS12381 is the private key of BLS using BLS12_381, it implements PrKey
type PrKeyECDSA struct {
	// the signature algo
	alg *ECDSAalgo
	// public key
	goecdsa.PrivateKey
}

// AlgoName returns the name of the algo related to the private key
func (sk *PrKeyECDSA) AlgoName() AlgoName {
	return sk.alg.Name()
}

// KeySize returns the length of the private key
func (sk *PrKeyECDSA) KeySize() int {
	return ((sk.alg.N).BitLen() + 7) / 8
}

// ComputePubKey computes the public key associated to the private key
func (sk *PrKeyECDSA) ComputePubKey() {
	var newPk PubKey_BLS_BLS12381
	// compute public key pk = g2^sk
	_G2scalarGenMult(&(newPk.point), &(sk.scalar))
	sk.pk = &newPk
}

// Pubkey returns the public key associated to the private key
func (sk *PrKeyECDSA) Pubkey() PubKey {
	if sk.pk != nil {
		return sk.pk
	}
	sk.ComputePubKey()
	return sk.pk
}

// PubKeyECDSA is the public key of ECDSA, it implements PubKey
type PubKeyECDSA goecdsa.PublicKey

// KeySize returns the length of the public key
func (pk *PubKeyECDSA) KeySize() int {
	return pubKeyLengthBLS_BLS12381
}
