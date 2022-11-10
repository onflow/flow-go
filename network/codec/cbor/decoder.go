// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package cbor

import (
	"fmt"

	"github.com/fxamacker/cbor/v2"

	"github.com/onflow/flow-go/network/codec"
)

// Decoder implements a stream decoder for CBOR.
type Decoder struct {
	dec *cbor.Decoder
}

// Decode will decode the next CBOR value from the stream.
// Expected error returns during normal operations:
//   - codec.ErrInvalidEncoding if message encoding is invalid.
//   - codec.ErrUnknownMsgCode if message code byte does not match any of the configured message codes.
//   - codec.ErrMsgUnmarshal if the codec fails to unmarshal the data to the message type denoted by the message code.
func (d *Decoder) Decode() (interface{}, error) {

	// read from stream and extract code
	var data []byte
	err := d.dec.Decode(&data)
	if err != nil {
		return nil, codec.NewInvalidEncodingErr(err)
	}

	if len(data) == 0 {
		return nil, codec.NewInvalidEncodingErr(fmt.Errorf("empty data"))
	}

	msgInterface, what, err := codec.InterfaceFromMessageCode(data[0])
	if err != nil {
		return nil, err
	}

	// unmarshal the payload
	err = defaultDecMode.Unmarshal(data[1:], msgInterface) // all but first byte
	if err != nil {
		return nil, codec.NewMsgUnmarshalErr(data[0], what, err)
	}

	return msgInterface, nil
}
