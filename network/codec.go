// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package network

import (
	"io"
)

// Codec provides factory functions for encoders and decoders.
type Codec interface {
	NewEncoder(w io.Writer) Encoder
	NewDecoder(r io.Reader) Decoder
	Encode(v interface{}) ([]byte, error)
	Decode(data []byte) (interface{}, error)
}

// Encoder encodes the given message into the underlying writer.
type Encoder interface {
	Encode(v interface{}) error
}

// Decoder decodes from the underlying reader into the given message.
type Decoder interface {
	Decode() (interface{}, error)
}
