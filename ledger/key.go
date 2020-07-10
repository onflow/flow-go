package ledger

import "bytes"

// Key represents a hierarchical ledger key
type Key struct {
	KeyParts []KeyPart
}

// NewKey construct a new key
func NewKey(kp []KeyPart) *Key {
	return &Key{KeyParts: kp}
}

// Size returns the byte size needed to encode the key
func (k *Key) Size() int {
	size := 0
	for _, kp := range k.KeyParts {
		// value size + 2 bytes for type
		size += len(kp.Value) + 2
	}
	return size
}

func (k *Key) String() string {
	// TODO include type
	ret := ""
	for _, kp := range k.KeyParts {
		ret += string(kp.Value)
	}
	return ret
}

// Equals compares this key to another key
func (k *Key) Equals(other *Key) bool {
	if other == nil {
		return false
	}
	if len(k.KeyParts) != len(other.KeyParts) {
		return false
	}
	for i, kp := range k.KeyParts {
		if !kp.Equals(&other.KeyParts[i]) {
			return false
		}
	}
	return true
}

// KeyPart is a typed part of a key
// TODO add docs on types ...
type KeyPart struct {
	Type  uint16
	Value []byte
}

// NewKeyPart construct a new key part
func NewKeyPart(typ uint16, val []byte) *KeyPart {
	return &KeyPart{Type: typ, Value: val}
}

// Equals compares this key part to another key part
func (kp *KeyPart) Equals(other *KeyPart) bool {
	if other == nil {
		return false
	}
	if kp.Type != other.Type {
		return false
	}
	return bytes.Equal(kp.Value, other.Value)
}
