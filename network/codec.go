package network

import (
	"io"

	"github.com/onflow/flow-go/model/messages"
)

// Codec provides factory functions for encoders and decoders.
type Codec interface {
	NewEncoder(w io.Writer) Encoder
	NewDecoder(r io.Reader) Decoder
	Encode(v any) ([]byte, error)

	// Decode decodes a message.
	// Expected error returns during normal operations:
	//  - codec.ErrInvalidEncoding if message encoding is invalid.
	//  - codec.ErrUnknownMsgCode if message code byte does not match any of the configured message codes.
	//  - codec.ErrMsgUnmarshal if the codec fails to unmarshal the data to the message type denoted by the message code.
	Decode(data []byte) (messages.UntrustedMessage, error)
}

// Encoder encodes the given message into the underlying writer.
type Encoder interface {
	Encode(v any) error
}

// Decoder decodes from the underlying reader into the given message.
// Expected error returns during normal operations:
//   - codec.ErrInvalidEncoding if message encoding is invalid.
//   - codec.ErrUnknownMsgCode if message code byte does not match any of the configured message codes.
//   - codec.ErrMsgUnmarshal if the codec fails to unmarshal the data to the message type denoted by the message code.
type Decoder interface {
	Decode() (messages.UntrustedMessage, error)
}
