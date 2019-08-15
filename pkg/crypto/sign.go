package crypto

import (
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

// NewSignatureAlgo initializes and chooses a signature scheme
func NewSignatureAlgo(name AlgoName) Signer {
	if name == BLS_BLS12381 {
		a := &(BLS_BLS12381Algo{
			nil,
			&SignAlgo{
				name,
				prKeyLengthBLS_BLS12381,
				pubKeyLengthBLS_BLS12381,
				signatureLengthBLS_BLS12381}})
		a.init()
		return a
	}
	log.Errorf("the signature scheme %s is not supported.", name)
	return nil
}

// Signer interface
type Signer interface {
	Name() AlgoName
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
	GeneratePrKey([]byte) PrKey
}

// SignAlgo
type SignAlgo struct {
	name            AlgoName
	PrKeyLength     int
	PubKeyLength    int
	SignatureLength int
}

// Name returns the name of the algorithm
func (a *SignAlgo) Name() AlgoName {
	return a.name
}

// SignatureSize returns the size of a signature in bytes
func (a *SignAlgo) SignatureSize() int {
	return a.SignatureLength
}

// Signature type tools

// ToBytes returns a byte array of the signature data
func (s Signature) ToBytes() []byte {
	return s[:]
}

// String returns a String representation of the signature data
func (s Signature) String() string {
	const zero = "00"
	var sb strings.Builder
	sb.WriteString("0x")
	for _, i := range s {
		hex := strconv.FormatUint(uint64(i), 16)
		sb.WriteString(zero[:2-len(hex)])
		sb.WriteString(hex)
	}
	return sb.String()
}

// Key Pair

// PrKey is an unspecified signature scheme private key
type PrKey interface {
	// returns the name of the algorithm related to the private key
	AlgoName() AlgoName
	// return the size in bytes
	KeySize() int
	// computes the pub key associated with the private key
	ComputePubKey()
	// returns the public key
	Pubkey() PubKey
}

// PubKey is an unspecified signature scheme public key
type PubKey interface {
	// return the size in bytes
	KeySize() int
}
