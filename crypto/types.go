package crypto

//revive:disable:var-naming

// SigningAlgorithm is an identifier for a signing algorithm and curve.
type SigningAlgorithm int

const (
	// Supported signing algorithms
	UnknownSigningAlgorithm SigningAlgorithm = iota
	BLS_BLS12381
	ECDSA_P256
	ECDSA_SECp256k1
)

// String returns the string representation of this signing algorithm.
func (f SigningAlgorithm) String() string {
	return [...]string{"UNKNOWN", "BLS_BLS12381", "ECDSA_P256", "ECDSA_SECp256k1"}[f]
}

// HashingAlgorithm is an identifier for a hashing algorithm.
type HashingAlgorithm int

const (
	// Supported hashing algorithms
	UnknownHashingAlgorithm HashingAlgorithm = iota
	SHA2_256
	SHA2_384
	SHA3_256
	SHA3_384
)

// String returns the string representation of this hashing algorithm.
func (f HashingAlgorithm) String() string {
	return [...]string{"UNKNOWN", "SHA2_256", "SHA2_384", "SHA3_256", "SHA3_384"}[f]
}

const (
	// Lengths of hash outputs in bytes
	HashLenSha2_256 = 32
	HashLenSha2_384 = 48
	HashLenSha3_256 = 32
	HashLenSha3_384 = 48

	// BLS signature scheme lengths

	// BLS12-381
	compression = 1 // 1 for compressed, 0 for uncompressed
	// the length is divided by 2 if compression is on
	SignatureLenBLS_BLS12381 = 48 * (2 - compression)
	PrKeyLenBLS_BLS12381     = 32
	// the length is divided by 2 if compression is on
	PubKeyLenBLS_BLS12381 = 96 * (2 - compression)

	// ECDSA

	// NIST P256
	SignatureLenECDSA_P256     = 64
	PrKeyLenECDSA_P256         = 32
	PubKeyLenECDSA_P256        = 64
	KeyGenSeedMinLenECDSA_P256 = 40

	// SEC p256k1
	SignatureLenECDSA_SECp256k1     = 64
	PrKeyLenECDSA_SECp256k1         = 32
	PubKeyLenECDSA_SECp256k1        = 64
	KeyGenSeedMinLenECDSA_SECp256k1 = 40

	DKGMinSize int = 3
	ThresholdMinSize
)

// Signature is a generic type, regardless of the signature scheme
type Signature []byte
