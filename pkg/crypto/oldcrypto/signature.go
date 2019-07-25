package oldcrypto

import (
	"encoding/hex"
)

type Signature interface {
	// ToBytes returns the bytes representation of a signature
	ToBytes() []byte
	// String returns a hex string representation of signature bytes
	String() string
}

type MockSignature []byte

func (s MockSignature) ToBytes() []byte {
	return s[:]
}

func (s MockSignature) String() string {
	return "0x" + hex.EncodeToString(s.ToBytes())
}

// Sign signs a digest with the provided key pair.
func Sign(digest Hash, keyPair *KeyPair) Signature {
	// TODO: implement real signatures
	return MockSignature(keyPair.PublicKey)
}
