package crypto

//revive:disable:var-naming

// SigningAlgorithm is an identifier for a signing algorithm and curve.
type SigningAlgorithm int

const (
	// Supported signing algorithms
	UnknownSigningAlgorithm SigningAlgorithm = iota
	BLS_BLS12381
	EcdsaP256
	EcdsaSecp256k1
)

// String returns the string representation of this signing algorithm.
func (f SigningAlgorithm) String() string {
	return [...]string{"UNKNOWN", "BLS_BLS12381", "EcdsaP256", "EcdsaSecp256k1"}[f]
}

const (
	// targeted bits of security
	securityBits = 128

	// BLS signature scheme lengths

	// BLS12-381
	// p size in bytes
	fieldSize = 48
	// Points compression: 1 for compressed, 0 for uncompressed
	compression = 1
	// the length is divided by 2 if compression is on
	SignatureLenBlsBls12381 = fieldSize * (2 - compression)
	PrKeyLenBlsBls12381     = 32
	// the length is divided by 2 if compression is on
	PubKeyLenBlsBls12381 = 2 * fieldSize * (2 - compression)
	// Input length of the optimized SwU map to G1: 2*(P_size+security)
	// security being 128 bits
	opSwUInputLenBlsBls12381    = 2 * (fieldSize + (securityBits / 8))
	KeyGenSeedMinLenBlsBls12381 = PrKeyLenBlsBls12381 + (securityBits / 8)

	// ECDSA

	// NIST P256
	SignatureLenEcdsaP256     = 64
	PrKeyLenEcdsaP256         = 32
	PubKeyLenEcdsaP256        = 64
	KeyGenSeedMinLenEcdsaP256 = PrKeyLenEcdsaP256 + (securityBits / 8)

	// SEC p256k1
	SignatureLenEcdsaSecp256k1     = 64
	PrKeyLenEcdsaSecp256k1         = 32
	PubKeyLenEcdsaSecp256k1        = 64
	KeyGenSeedMinLenEcdsaSecp256k1 = PrKeyLenEcdsaSecp256k1 + (securityBits / 8)

	// DKG and Threshold Signatures
	DKGMinSize       int = 3
	ThresholdMinSize     = DKGMinSize
	DKGMaxSize       int = 254
	ThresholdMaxSize     = DKGMaxSize
	SeedMinLenDKG        = securityBits / 8
)

// Signature is a generic type, regardless of the signature scheme
type Signature []byte
