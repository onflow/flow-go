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
var BLS_BLS12381Once sync.Once
var ECDSA_P256Once sync.Once
var ECDSA_SECp256k1Once sync.Once

// NewSignatureAlgo initializes and chooses a signature scheme
func NewSigner(name AlgoName) (Signer, error) {
	if name == BLS_BLS12381 {
		BLS_BLS12381Once.Do(func() {
			BLS_BLS12381Instance = &(BLS_BLS12381Algo{
				commonSigner: &commonSigner{
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
		ECDSA_P256Once.Do(func() {
			ECDSA_P256Instance = &(ECDSAalgo{
				curve: elliptic.P256(),
				commonSigner: &commonSigner{
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
		ECDSA_SECp256k1Once.Do(func() {
			ECDSA_SECp256k1Instance = &(ECDSAalgo{
				curve: secp256k1(),
				commonSigner: &commonSigner{
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
	// prKeySize() returns the private key length in bytes
	prKeySize() int
	// Private key functions
	GeneratePrKey([]byte) (PrivateKey, error)
	// DecodePrKey loads a private key from a byte array
	DecodePrKey([]byte) (PrivateKey, error)
	// DecodePubKey loads a public key from a byte array
	DecodePubKey([]byte) (PublicKey, error)
}

// commonSigner holds the common data for all signers
type commonSigner struct {
	name            AlgoName
	PrKeyLength     int
	PubKeyLength    int
	SignatureLength int
}

// Name returns the name of the algorithm
func (a *commonSigner) Name() AlgoName {
	return a.name
}

// SignatureSize returns the size of a signature in bytes
func (a *commonSigner) SignatureSize() int {
	return a.SignatureLength
}

func (a *commonSigner) prKeySize() int {
	return a.PrKeyLength
}

// Signature type tools

// Bytes returns a byte array of the signature data
func (s Signature) Bytes() []byte {
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

// PrivateKey is an unspecified signature scheme private key
type PrivateKey interface {
	// returns the name of the algorithm related to the private key
	AlgoName() AlgoName
	// return the size in bytes
	KeySize() int
	// Signature generation function
	Sign([]byte, Hasher) (Signature, error)
	// returns the public key
	Pubkey() PublicKey
	// Encode returns a bytes representation of the private key
	Encode() ([]byte, error)
}

// PublicKey is an unspecified signature scheme public key
type PublicKey interface {
	// returns the name of the algorithm related to the public key
	AlgoName() AlgoName
	// return the size in bytes
	KeySize() int
	// Signature verification function
	Verify(Signature, []byte, Hasher) (bool, error)
	// Encode returns a bytes representation of the public key
	Encode() ([]byte, error)
}
