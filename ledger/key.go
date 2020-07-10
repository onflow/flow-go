package ledger

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
