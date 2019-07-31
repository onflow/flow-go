package crypto

// AlgoName is the supported algos type
type AlgoName string

const (
	// Hashing supported algorithms
	SHA3_256 AlgoName = "SHA3_256"

	// Signing supported algorithms
	BLS_BLS12381 = "BLS_BLS12381"
)

const (
	// Lengths of hash outputs in bytes
	HashLengthSha2_256 = 32
	HashLengthSha3_256 = 32
	HashLengthSha3_512 = 64

	// BLS signature scheme lengths

	// BLS12-381
	SignatureLengthBLS_BLS12381 = 48
	PrKeyLengthBLS_BLS12381     = 32
	PubKeyLengthBLS_BLS12381    = 96
)

// These types should implement Hash

// Hash32 is 256-bits digest
type Hash32 [32]byte

// Hash64 is 512-bits digest
type Hash64 [64]byte

// Signature is a generic type, regardless of the signature scheme
type Signature []byte
