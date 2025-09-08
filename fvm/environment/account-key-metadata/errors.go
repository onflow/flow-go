package accountkeymetadata

import (
	"fmt"
)

type CollisionError struct {
	key1   []byte
	key2   []byte
	digest uint64
}

func NewCollisionError(encodedKey1, encodedKey2 []byte, digest uint64) *CollisionError {
	return &CollisionError{
		key1:   encodedKey1,
		key2:   encodedKey2,
		digest: digest,
	}
}

func (e *CollisionError) Error() string {
	return fmt.Sprintf("found public key digest collision: keys %x and %x have the same digest %d", e.key1, e.key2, e.digest)
}
