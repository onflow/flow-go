// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package network

import (
	"io"
)

// Codec provides factory functions for encoders and decoders and utility functions
// for encoding and decoding.
type Codec interface {
	// NewEncoder returns a new encoder instance which encodes into the input writer w.
	NewEncoder(w io.Writer) Encoder
	// NewDecoder returns a new decoder instance which decodes into the input reader r.
	NewDecoder(r io.Reader) Decoder
	// Encode encodes the input value and returns the encoded bytes.
	// No errors are expected during normal operation
	Encode(v interface{}) ([]byte, error)
	// Decode decodes from the input bytes and returns the decoded value.
	// Expected error returns during normal operations:
	//   - codec.ErrUnknownMsgCode if message code byte does not match any of the configured message codes.
	//   - codec.ErrMsgUnmarshal if the codec fails to unmarshal the data to the message type denoted by the message code.
	Decode(data []byte) (interface{}, error)
}

// Encoder encodes the given message into the underlying writer.
type Encoder interface {
	// Encode encodes the input value into the underlying writer.
	// No errors are expected during normal operation
	Encode(v interface{}) error
}

// Decoder decodes from the underlying reader into the given message.
// Decoder implementations must enforce structural validity for any decoded Go types
// which implement model.StructureValidator.
// Expected error returns during normal operations:
//   - codec.ErrUnknownMsgCode if message code byte does not match any of the configured message codes.
//   - codec.ErrMsgUnmarshal if the codec fails to unmarshal the data to the message type denoted by the message code.
type Decoder interface {
	// Decode decodes a message and validates the resulting type's structural validity, if possible.
	// Expected error returns during normal operations:
	//  * codec.ErrUnknownMsgCode if message code byte does not match any of the configured message codes.
	//  * codec.ErrMsgUnmarshal if the codec fails to unmarshal the data to the message type denoted by the message code.
	Decode() (interface{}, error)
}
