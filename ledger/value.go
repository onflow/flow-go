package ledger

import "encoding/hex"

// Value holds the value part of a ledger key value pair
type Value []byte

// Size returns the value size
func (v Value) Size() int {
	return len(v)
}

// Encode Encodes the value
// This has been considered for future extensions
// in case we decide to convert it to a full structure
func (v Value) Encode() []byte {
	return v
}

func (v Value) String() string {
	return hex.EncodeToString(v)
}

// DecodeValue constructs a value from an encoded value
// This has been considered for future extensions
func DecodeValue(encodedValue []byte) Value {
	return Value(encodedValue)
}
