package crypto

import (
	"crypto/elliptic"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

var BLS_BLS12381Instance *BLS_BLS12381Algo
var ECDSA_P256Instance *ECDSAalgo
var ECDSA_SECp256k1Instance *ECDSAalgo

//  Once variables to make sure each Signer is instanciated only once
var once [3]sync.Once

// NewSignatureAlgo initializes and chooses a signature scheme
func NewSignatureAlgo(name AlgoName) (Signer, error) {
	if name == BLS_BLS12381 {
		once[0].Do(func() {
			BLS_BLS12381Instance = &(BLS_BLS12381Algo{
				SignAlgo: &SignAlgo{
					name,
					prKeyLengthBLS_BLS12381,
					pubKeyLengthBLS_BLS12381,
					signatureLengthBLS_BLS12381,
				},
			})
			BLS_BLS12381Instance.init()
		})
		return BLS_BLS12381Instance, nil
	}

	if name == ECDSA_P256 {
		once[1].Do(func() {
			ECDSA_P256Instance = &(ECDSAalgo{
				curve: elliptic.P256(),
				SignAlgo: &SignAlgo{
					name,
					PrKeyLengthECDSA_P256,
					PubKeyLengthECDSA_P256,
					SignatureLengthECDSA_P256,
				},
			})
		})
		return ECDSA_P256Instance, nil
	}

	if name == ECDSA_SECp256k1 {
		once[2].Do(func() {
			ECDSA_SECp256k1Instance = &(ECDSAalgo{
				curve: secp256k1(),
				SignAlgo: &SignAlgo{
					name,
					PrKeyLengthECDSA_SECp256k1,
					PubKeyLengthECDSA_SECp256k1,
					SignatureLengthECDSA_SECp256k1,
				},
			})
		})
		return ECDSA_SECp256k1Instance, nil
	}
	return nil, cryptoError{fmt.Sprintf("the signature scheme %s is not supported.", name)}
}

// Signer interface
type Signer interface {
	Name() AlgoName
	// Size returns the signature output length in bytes
	SignatureSize() int
	// PrKeySize() returns the private key length in bytes
	PrKeySize() int
	// Signature functions
	SignHash(PrKey, Hash) (Signature, error)
	SignBytes(PrKey, []byte, Hasher) (Signature, error)
	SignStruct(PrKey, Encoder, Hasher) (Signature, error)
	// Verification functions
	VerifyHash(PubKey, Signature, Hash) (bool, error)
	VerifyBytes(PubKey, Signature, []byte, Hasher) (bool, error)
	VerifyStruct(PubKey, Signature, Encoder, Hasher) (bool, error)
	// Private key functions
	GeneratePrKey([]byte) (PrKey, error)
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

func (a *SignAlgo) PrKeySize() int {
	return a.PrKeyLength
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
	// returns the signer structure associated to the private key
	Signer() Signer
	// return the size in bytes
	KeySize() int
	// returns the public key
	Pubkey() PubKey
}

// PubKey is an unspecified signature scheme public key
type PubKey interface {
	// return the size in bytes
	KeySize() int
}
