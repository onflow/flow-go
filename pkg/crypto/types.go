package crypto

// Lengths of hashes in bytes.
const (
	HashLengthSha3_256 = 32
	HashLength         = HashLengthSha3_256
)

// Hash represents the 32 byte SHA3-256 output of arbitrary data.
// Eventually, this will change to dynamic arrays to support multiple hash algorithms and crypto agility
type Hash [HashLength]byte
