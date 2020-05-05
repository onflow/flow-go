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

// StringToHashAlgorithm converts a string to a HashAlgorithm.
func StringToHashAlgorithm(s string) HashingAlgorithm {
	switch s {
	case SHA2_256.String():
		return SHA2_256
	case SHA2_384.String():
		return SHA2_384
	case SHA3_256.String():
		return SHA3_256
	case SHA3_384.String():
		return SHA3_384
	default:
		return UnknownHashingAlgorithm
	}
}

const (

	// Lengths of hash outputs in bytes
	HashLenSha2_256 = 32
	HashLenSha2_384 = 48
	HashLenSha3_256 = 32
	HashLenSha3_384 = 48

	// KMAC
	// the parameter maximum bytes-length as defined in NIST SP 800-185
	KmacMaxParamsLen = 2040 / 8
)
