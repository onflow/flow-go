package hash

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// HashLen is the ledger default output hash length in bytes
const HashLen = 32

// Hash is the hash type used in all ledger
type Hash [HashLen]byte

func (h Hash) String() string {
	return hex.EncodeToString(h[:])
}

func (h Hash) MarshalJSON() ([]byte, error) {
	return json.Marshal(hex.EncodeToString(h[:]))
}

func (h *Hash) UnmarshalJSON(data []byte) error {
	var hashHex string
	if err := json.Unmarshal(data, &hashHex); err != nil {
		return err
	}
	b, err := hex.DecodeString(hashHex)
	if err != nil {
		return err
	}
	ha, err := ToHash(b)
	if err != nil {
		return err
	}
	*h = ha
	return nil
}

// DummyHash is an arbitrary hash value, used in function errors.
// DummyHash represents a valid hash value.
var DummyHash Hash

// HashLeaf returns the hash value for leaf nodes. Path is a fixed-length
// byte array which should be holding exactly 32 bytes. Note that we don't
// include the keys here as they are already included in the path.
func HashLeaf(path Hash, value []byte) Hash {
	hasher := &state{}
	return hasher.hash256Plus(path, value) // path is 256 bits
}

// HashInterNode returns the hash value for intermediate nodes. hash1 and hash2
// are fixed-length byte arrays which should be holding exactly 32 bytes each.
func HashInterNode(hash1 Hash, hash2 Hash) Hash {
	hasher := &state{}
	return hasher.hash256plus256(hash1, hash2) // hash1 and hash2 are 256 bits
}

// ToHash converts a byte slice to Hash (fixed-length byte array).
// It returns an error if the slice has an invalid length.
func ToHash(bytes []byte) (Hash, error) {
	var h Hash
	if len(bytes) != len(h) {
		return DummyHash, fmt.Errorf("expecting %d bytes but got %d bytes", len(h), len(bytes))
	}
	copy(h[:], bytes)
	return h, nil
}
