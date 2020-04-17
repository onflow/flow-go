package crypto

//revive:disable:var-naming

// SigningAlgorithm is an identifier for a signing algorithm
// (and parameters if applicable)
type SigningAlgorithm int

const (
	// Supported signing algorithms

	unknownSigningAlgorithm SigningAlgorithm = iota
	// BlsBls12381 is BLS on BLS 12-381 curve
	BlsBls12381
	// EcdsaP256 is ECDSA on NIST P-256 curve
	EcdsaP256
	// EcdsaSecp256k1 is ECDSA on SECp256k1 curve
	EcdsaSecp256k1
)

// String returns the string representation of this signing algorithm.
func (f SigningAlgorithm) String() string {
	return [...]string{"UNKNOWN", "BLS_BLS12381", "ECDSA_P256", "ECDSA_SECp256k1"}[f]
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
	// SignatureLenBlsBls12381 is the size of G1 elements
	SignatureLenBlsBls12381 = fieldSize * (2 - compression) // the length is divided by 2 if compression is on
	PrKeyLenBlsBls12381     = 32
	// PubKeyLenBlsBls12381 is the size of G2 elements
	PubKeyLenBlsBls12381 = 2 * fieldSize * (2 - compression) // the length is divided by 2 if compression is on
	// opSwUInputLenBlsBls12381 is the input length of the optimized SwU map to G1
	opSwUInputLenBlsBls12381    = 2 * (fieldSize + (securityBits / 8))
	KeyGenSeedMinLenBlsBls12381 = PrKeyLenBlsBls12381 + (securityBits / 8)
	KeyGenSeedMaxLenBlsBls12381 = maxScalarSize

	// ECDSA

	// NIST P256
	SignatureLenEcdsaP256 = 64
	PrKeyLenEcdsaP256     = 32
	// PubKeyLenEcdsaP256 is the size of uncompressed points on P256
	PubKeyLenEcdsaP256        = 64
	KeyGenSeedMinLenEcdsaP256 = PrKeyLenEcdsaP256 + (securityBits / 8)

	// SEC p256k1
	SignatureLenEcdsaSecp256k1 = 64
	PrKeyLenEcdsaSecp256k1     = 32
	// PubKeyLenEcdsaSecp256k1 is the size of uncompressed points on P256
	PubKeyLenEcdsaSecp256k1        = 64
	KeyGenSeedMinLenEcdsaSecp256k1 = PrKeyLenEcdsaSecp256k1 + (securityBits / 8)

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
)

// Signature is a generic type, regardless of the signature scheme
type Signature []byte
