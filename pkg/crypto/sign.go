package crypto

import (
	log "github.com/sirupsen/logrus"
)

// InitSignatureAlgo initializes and chooses a signature scheme
//-----------------------------------------------
func InitSignatureAlgo(name string) Signer {
	if name == "BLS_BLS12381" {
		s := &(BLS_BLS12381Algo{&SignAlgo{
			name,
			PrKeyLengthBLS_BLS12381,
			PubKeyLengthBLS_BLS12381,
			SignatureLengthBLS_BLS12381}})
		return s
	}
	log.Errorf("the signature algorithm %x is not supported", name)
	return nil
}

// Signer interface
//-----------------------------------------------
type Signer interface {
	GetName() string
	// Size return the signature output length
	SignatureSize() int
	// Signature functions
	SignHash(PrKey, Hash) Signature
	SignBytes(PrKey, []byte, Hasher) Signature
	SignStruct(PrKey, Encoder, Hasher) Signature
	// Verification functions
	VerifyHash(PubKey, Signature, Hash) bool
	VerifyBytes(PubKey, Signature, []byte, Hasher) bool
	VerifyStruct(PubKey, Signature, Encoder, Hasher) bool
	// Private key functions
	GeneratePrKey() PrKey // to be changed to accept a randomness source as an input
}

// SignAlgo implements Signer
//-----------------------------------------------
type SignAlgo struct {
	name            string
	PrKeyLength     int
	PubKeyLength    int
	SignatureLength int
}

// GetName returns the name of the algorithm
func (a *SignAlgo) GetName() string {
	return a.name
}

// SignatureSize returns the size of a signature in bytes
func (a *SignAlgo) SignatureSize() int {
	return a.SignatureLength
}

// SignHash is an obsolete function that gets overritten
func (a *SignAlgo) SignHash(sk PrKey, h Hash) Signature {
	var s Signature
	return s
}

// SignBytes signs an array of bytes
func (a *SignAlgo) SignBytes(sk PrKey, data []byte, alg Hasher) Signature {
	h := alg.ComputeBytesHash(data)
	return a.SignHash(sk, h)
}

// SignStruct signs a structure
func (a *SignAlgo) SignStruct(sk PrKey, data Encoder, alg Hasher) Signature {
	h := alg.ComputeStructHash(data)
	return a.SignHash(sk, h)
}

// SignHash is an obsolete function that gets overritten
func (a *SignAlgo) VerifyHash(pk PubKey, s Signature, h Hash) bool {
	return true
}

// VerifyBytes verifies a signature of a byte array
func (a *SignAlgo) VerifyBytes(pk PubKey, s Signature, data []byte, alg Hasher) bool {
	h := alg.ComputeBytesHash(data)
	return a.VerifyHash(pk, s, h)
}

// VerifyStruct verifies a signature of a structure
func (a *SignAlgo) VerifyStruct(pk PubKey, s Signature, data Encoder, alg Hasher) bool {
	h := alg.ComputeStructHash(data)
	return a.VerifyHash(pk, s, h)
}

// GeneratePrKey is an obsolete function that gets overritten
func (a *SignAlgo) GeneratePrKey() PrKey {
	var sk PrKey
	return sk
}

// Signature type tools
//----------------------
// Signature is unspecified signature scheme signature
type Signature interface {
	// ToBytes returns the bytes representation of a signature
	ToBytes() []byte
	// String returns a hex string representation of signature bytes
	String() string
}

// Key Pair
//----------------------
// PrKey is an unspecified signature scheme private key
type PrKey interface {
	// returns the name of the algorithm related to the private key
	GetAlgoName() string
	// return the size in bytes
	GetKeySize() int
	// computes the pub key associated with the private key
	ComputePubKey()
	// returns the public key
	GetPubkey() PubKey
}

// PubKey is an unspecified signature scheme public key
type PubKey interface {
	// return the size in bytes
	GetKeySize() int
}
