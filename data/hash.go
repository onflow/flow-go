package data

const (
	HashLength = 32
)

// Hash represents the 32 byte Keccak256 hash of arbitrary data.
type Hash [HashLength]byte
