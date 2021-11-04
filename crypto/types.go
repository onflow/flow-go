package crypto

import (
	"errors"
	"fmt"
)

//revive:disable:var-naming

// the `go generate` command requires bash scripting, `cmake` and `git`.
//go:generate bash ./build_dependency.sh

// SigningAlgorithm is an identifier for a signing algorithm
// (and parameters if applicable)
type SigningAlgorithm int

const (
	// Supported signing algorithms
	UnknownSigningAlgorithm SigningAlgorithm = iota
	// BLSBLS12381 is BLS on BLS 12-381 curve
	BLSBLS12381
	// ECDSAP256 is ECDSA on NIST P-256 curve
	ECDSAP256
	// ECDSASecp256k1 is ECDSA on secp256k1 curve
	ECDSASecp256k1
)

// String returns the string representation of this signing algorithm.
func (f SigningAlgorithm) String() string {
	return [...]string{"UNKNOWN", "BLS_BLS12381", "ECDSA_P256", "ECDSA_secp256k1"}[f]
}

const (
	// Minimum targeted bits of security.
	// This is used as a reference but it doesn't mean all implemented primitives provide this minimum.
	securityBits = 128

	// BLS signature scheme lengths

	// BLS12-381
	// p size in bytes, where G1 is defined over the field Zp
	fieldSize = 48
	//
	// 1 for compressed, 0 for uncompressed - values should not be changed
	uncompressed = 0
	compressed   = 1
	// Points compression when serialized
	serializationG1 = compressed
	serializationG2 = compressed
	//
	// SignatureLenBLSBLS12381 is the size of G1 elements
	SignatureLenBLSBLS12381 = fieldSize * (2 - serializationG1) // the length is divided by 2 if compression is on
	PrKeyLenBLSBLS12381     = 32
	// PubKeyLenBLSBLS12381 is the size of G2 elements
	PubKeyLenBLSBLS12381        = 2 * fieldSize * (2 - serializationG2) // the length is divided by 2 if compression is on
	KeyGenSeedMinLenBLSBLS12381 = PrKeyLenBLSBLS12381 + (securityBits / 8)
	KeyGenSeedMaxLenBLSBLS12381 = maxScalarSize
	// opSwUInputLenBLSBLS12381 is the input length of the optimized SwU map to G1
	opSwUInputLenBLSBLS12381 = 2 * (fieldSize + (securityBits / 8))
	// minimum output size as required by the chosen implementation of hash to curve
	minHashSizeBLSBLS12381 = opSwUInputLenBLSBLS12381
	// Cipher suite with all the settings
	blsCipherSuite = "BLS_SIG_BLS12381G1_XOF:KMAC128_SSWU_RO_POP_"

	// ECDSA

	KeyGenSeedMaxLenECDSA = 2048 // large enough constant accepted by the implementation

	// NIST P256
	SignatureLenECDSAP256 = 64
	PrKeyLenECDSAP256     = 32
	// PubKeyLenECDSAP256 is the size of uncompressed points on P256
	PubKeyLenECDSAP256        = 64
	KeyGenSeedMinLenECDSAP256 = PrKeyLenECDSAP256 + (securityBits / 8)

	// SECG secp256k1
	SignatureLenECDSASecp256k1 = 64
	PrKeyLenECDSASecp256k1     = 32
	// PubKeyLenECDSASecp256k1 is the size of uncompressed points on P256
	PubKeyLenECDSASecp256k1        = 64
	KeyGenSeedMinLenECDSASecp256k1 = PrKeyLenECDSASecp256k1 + (securityBits / 8)

	// DKG and Threshold Signatures

	// MinimumThreshold is the minimum value of the threshold parameter in all threshold-based protocols.
	MinimumThreshold = 1
	// DKGMinSize is the minimum size of a group participating in a DKG protocol
	DKGMinSize int = MinimumThreshold + 1
	// DKGMaxSize is the maximum size of a group participating in a DKG protocol
	DKGMaxSize int = 254
	// SeedMinLenDKG is the minumum seed length required to participate in a DKG protocol
	SeedMinLenDKG = securityBits / 8
	SeedMaxLenDKG = maxRelicPrgSeed
	// ThresholdSignMinSize is the minimum size of a group participating in a threshold signature protocol
	ThresholdSignMinSize = MinimumThreshold + 1
	// ThresholdSignMaxSize is the maximum size of a group participating in a threshold signature protocol
	ThresholdSignMaxSize = DKGMaxSize

	// Relic internal constant (related to exported constants above)
	// max byte length of bn_st set to 2048 bits
	maxScalarSize = 256

	// max relic PRG seed length in bytes
	maxRelicPrgSeed = 1 << 32
)

// Signature is a generic type, regardless of the signature scheme
type Signature []byte

// InvalidInputsError is an error returned when a crypto API receives invalid inputs.
// It allows a function caller differentiate unexpected program errors from errors caused by
// invalid inputs.
type InvalidInputsError struct {
	message string
}

// invalidInputsErrorf constructs a new InvalidInputsError
func invalidInputsErrorf(msg string, args ...interface{}) error {
	return &InvalidInputsError{message: fmt.Sprintf(msg, args...)}
}

func (e InvalidInputsError) Error() string {
	return e.message
}

// IsInvalidInputsError checks if the input error is of a InvalidInputsError type
func IsInvalidInputsError(err error) bool {
	var target *InvalidInputsError
	return errors.As(err, &target)
}
