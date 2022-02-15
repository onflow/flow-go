package encoding

import (
	"io"
)

// Encodable is a type that defines a canonical encoding.
type Encodable interface {
	Encode() []byte
}

// Marshaler marshals and unmarshals values to and from bytes.
type Marshaler interface {
	// Marshaler marshals a value to bytes.
	//
	// This function returns an error if the value type is not supported by this marshaler.
	Marshal(interface{}) ([]byte, error)

	// Unmarshal unmarshals bytes to a value.
	//
	// This functions returns an error if the bytes do not fit the provided value type.
	Unmarshal([]byte, interface{}) error

	// MustMarshal marshals a value to bytes.
	//
	// This function panics if marshaling fails.
	MustMarshal(interface{}) []byte

	// MustUnmarshal unmarshals bytes to a value.
	//
	// This function panics if decoding fails.
	MustUnmarshal([]byte, interface{})
}

type Encoder interface {
	Encode(interface{}) error
}

type Decoder interface {
	Decode(interface{}) error
}

type Codec interface {
	NewEncoder(w io.Writer) Encoder
	NewDecoder(r io.Reader) Decoder
}
