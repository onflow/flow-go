package crypto

// Lengths of hash outputs in bytes.
const (
	HashLengthSha2_256 = 32
	HashLengthSha3_256 = 32
	HashLengthSha3_512 = 64

	HashLength = 32
)

// Hash represents the hash algorithms output types
/*type Hash interface {
	ToBytes() []byte
	//Len() int
}*/
type Hash [32]byte

// These types should implement Hash

// Hash16 is 128-bits digest
type Hash16 [16]byte

// Hash32 is 256-bits digest
type Hash32 [32]byte

// Hash64 is 512-bits digest
type Hash64 [64]byte
