package hash

//revive:disable:var-naming

// HashingAlgorithm is an identifier for a hashing algorithm.
type HashingAlgorithm int

const (
	// Supported hashing algorithms
	UnknownHashingAlgorithm HashingAlgorithm = iota
	SHA2_256
	SHA2_384
	SHA3_256
	SHA3_384
	KMAC128
)

// String returns the string representation of this hashing algorithm.
func (f HashingAlgorithm) String() string {
	return [...]string{"UNKNOWN", "SHA2_256", "SHA2_384", "SHA3_256", "SHA3_384", "KMAC128"}[f]
}

const (
	// minimum targeted bits of security
	securityBits = 128

	// Lengths of hash outputs in bytes
	HashLenSHA2_256 = 32
	HashLenSHA2_384 = 48
	HashLenSHA3_256 = 32
	HashLenSHA3_384 = 48

	// KMAC
	// the minimum key length in bytes
	KmacMinKeyLen = securityBits / 8
)
