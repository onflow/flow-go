package ledger

import "encoding/binary"

// Key represents a hierarchical ledger key
type Key struct {
	KeyParts []KeyPart
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

// Encode encodes a key into a byte slice
func (k *Key) Encode() []byte {
	// TODO RAMTIN fix me
	ret := make([]byte, 0)
	// encode number of key parts
	kpSize := make([]byte, 2)
	binary.BigEndian.PutUint16(kpSize, uint16(len(k.KeyParts)))
	ret = append(ret, kpSize...)

	for _, kp := range k.KeyParts {
		encKP := kp.Encode()
		// 2 byte len of key part
		encKPSize := make([]byte, 2)
		binary.BigEndian.PutUint16(encKPSize, uint16(len(encKP)))
		ret = append(ret, encKPSize...)
		ret = append(ret, encKP...)
	}
	return ret
}

// DecodeKey constructs a key from an encoded key part
// TODO add errors
func DecodeKey(encodedKey []byte) *Key {
	key := &Key{}
	numOfParts := binary.BigEndian.Uint16(encodedKey[:2])
	lastIndex := 2
	for i := 0; i < int(numOfParts); i++ {
		kpEncSize := binary.BigEndian.Uint16(encodedKey[lastIndex : lastIndex+2])
		lastIndex += 2
		kpEnc := encodedKey[lastIndex : lastIndex+int(kpEncSize)]
		lastIndex += int(kpEncSize)
		key.KeyParts = append(key.KeyParts, *DecodeKeyPart(kpEnc))
	}
	return key
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

// Encode encodes a key part into a byte slice
func (kp *KeyPart) Encode() []byte {
	ret := make([]byte, 0)
	// encode the type (first two bytes)
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, kp.Type)
	ret = append(ret, b...)
	// the rest is value
	ret = append(ret, kp.Value...)
	return ret
}

// DecodeKeyPart constructs a key part from an encoded key part
func DecodeKeyPart(encodedKeyPart []byte) *KeyPart {
	t := binary.BigEndian.Uint16(encodedKeyPart[:2])
	v := encodedKeyPart[2:]
	return &KeyPart{Type: t, Value: v}
}
