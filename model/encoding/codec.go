package encoding

import (
	"github.com/onflow/flow-go/model/encoding/json"
)

// Encodable is a type that defines a canonical encoding.
type Encodable interface {
	Encode() []byte
}

// Encoder encodes and decodes values to and from bytes.
type Encoder interface {
	// Encode encodes a value as bytes.
	//
	// This function returns an error if the value type is not supported by this encoder.
	Encode(interface{}) ([]byte, error)

	// Decode decodes bytes into a value.
	//
	// This functions returns an error if the bytes do not fit the provided value type.
	Decode([]byte, interface{}) error

	// MustEncode encodes a value as bytes.
	//
	// This functions panic if encoding fails.
	MustEncode(interface{}) []byte

	// MustDecode decodes bytes into a value.
	//
	// This functions panic if decoding fails.
	MustDecode([]byte, interface{})
}

// DefaultEncoder is the default encoder used by Flow.
var DefaultEncoder Encoder = json.NewEncoder()
