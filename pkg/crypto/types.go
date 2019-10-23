package crypto

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
	return [...]string{"BLS_BLS12381", "ECDSA_P256", "ECDSA_SECp256k1"}[f]
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
	return [...]string{"SHA2_256", "SHA2_384", "SHA3_256", "SHA3_384"}[f]
}

const (
	// Lengths of hash outputs in bytes
	HashLengthSha2_256 = 32
	HashLengthSha2_384 = 48
	HashLengthSha3_256 = 32
	HashLengthSha3_384 = 48

	// BLS signature scheme lengths

	// BLS12-381
	compression = 1 // 1 for compressed, 0 for uncompressed
	// the length is divided by 2 if compression is on
	SignatureLengthBLS_BLS12381 = 48 * (2 - compression)
	PrKeyLengthBLS_BLS12381     = 32
	// the length is divided by 2 if compression is on
	PubKeyLengthBLS_BLS12381 = 96 * (2 - compression)

	// ECDSA

	// NIST P256
	SignatureLengthECDSA_P256 = 64
	PrKeyLengthECDSA_P256     = 32
	PubKeyLengthECDSA_P256    = 64

	// SEC p256k1
	SignatureLengthECDSA_SECp256k1 = 64
	PrKeyLengthECDSA_SECp256k1     = 32
	PubKeyLengthECDSA_SECp256k1    = 64
)

// Signature is a generic type, regardless of the signature scheme
type Signature []byte
