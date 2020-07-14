package crypto

//revive:disable:var-naming

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

// StringToSigningAlgorithm converts a string to a SigningAlgorithm.
func StringToSigningAlgorithm(s string) SigningAlgorithm {
	switch s {
	case BLSBLS12381.String():
		return BLSBLS12381
	case ECDSAP256.String():
		return ECDSAP256
	case ECDSASecp256k1.String():
		return ECDSASecp256k1
	default:
		return UnknownSigningAlgorithm
	}
}

const (
	// minimum targeted bits of security
	securityBits = 128

	// BLS signature scheme lengths

	// BLS12-381
	// p size in bytes, where G1 is defined over the field Zp
	fieldSize = 48
	// Points compression: 1 for compressed, 0 for uncompressed
	compression = 1
	// SignatureLenBLSBLS12381 is the size of G1 elements
	SignatureLenBLSBLS12381 = fieldSize * (2 - compression) // the length is divided by 2 if compression is on
	PrKeyLenBLSBLS12381     = 32
	// PubKeyLenBLSBLS12381 is the size of G2 elements
	PubKeyLenBLSBLS12381 = 2 * fieldSize * (2 - compression) // the length is divided by 2 if compression is on
	// opSwUInputLenBLSBLS12381 is the input length of the optimized SwU map to G1
	opSwUInputLenBLSBLS12381    = 2 * (fieldSize + (securityBits / 8))
	KeyGenSeedMinLenBLSBLS12381 = PrKeyLenBLSBLS12381 + (securityBits / 8)
	KeyGenSeedMaxLenBLSBLS12381 = maxScalarSize

	// ECDSA

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

	// DKGMinSize is the minimum size of a group participating in a DKG protocol
	DKGMinSize int = 1
	// DKGMaxSize is the minimum size of a group participating in a DKG protocol
	DKGMaxSize int = 254
	// SeedMinLenDKG is the minumum seed length required to participate in a DKG protocol
	SeedMinLenDKG = securityBits / 8
	SeedMaxLenDKG = maxRelicPrgSeed
	// ThresholdMinSize is the minimum size of a group participating in a threshold signature protocol
	ThresholdMinSize = DKGMinSize
	// ThresholdMaxSize is the minimum size of a group participating in a threshold signature protocol
	ThresholdMaxSize = DKGMaxSize

	// Relic internal constant (related to exported constants above)
	// max byte length of bn_st set to 2048 bits
	maxScalarSize = 256

	// max relic PRG seed length in bytes
	maxRelicPrgSeed = 1 << 32
)

// Signature is a generic type, regardless of the signature scheme
type Signature []byte
