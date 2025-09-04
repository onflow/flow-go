package accountkeymetadata

import (
	"fmt"
)

type KeyMetadataMalfromedError struct {
	msg string
}

func NewKeyMetadataMalfromedError(msg string) *KeyMetadataMalfromedError {
	return &KeyMetadataMalfromedError{msg}
}

func (e *KeyMetadataMalfromedError) Error() string {
	return fmt.Sprintf("key metadata is malformed: %s", e.msg)
}

type KeyMetadataNotFoundError struct {
	msg      string
	keyIndex uint32
}

func NewKeyMetadataNotFoundError(msg string, keyIndex uint32) *KeyMetadataNotFoundError {
	return &KeyMetadataNotFoundError{
		msg:      msg,
		keyIndex: keyIndex,
	}
}

func (e *KeyMetadataNotFoundError) Error() string {
	return fmt.Sprintf("key metadata not found for key index %d: %s", e.keyIndex, e.msg)
}

type UnexpectedKeyIndexError struct {
	index uint32
}

func NewUnexpectedKeyIndexError(index uint32) *UnexpectedKeyIndexError {
	return &UnexpectedKeyIndexError{index}
}

func (e *UnexpectedKeyIndexError) Error() string {
	return fmt.Sprintf("found unexpected key index %d", e.index)
}

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
