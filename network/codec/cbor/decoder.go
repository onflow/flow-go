// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package cbor

import (
	"fmt"

	"github.com/fxamacker/cbor/v2"

	"github.com/onflow/flow-go/model"
	"github.com/onflow/flow-go/network/codec"
	_ "github.com/onflow/flow-go/utils/binstat"
)

// Decoder implements a stream decoder for CBOR.
type Decoder struct {
	dec *cbor.Decoder
}

// Decode will decode the next CBOR value from the stream.
// If the CBOR value decodes to a Go type which implements model.StructureValidator
// this function will validate the Go type's structure using this method.
// Expected error returns during normal operations:
//   - codec.ErrUnknownMsgCode if message code byte does not match any of the configured message codes.
//   - codec.ErrMsgUnmarshal if the codec fails to unmarshal the data to the message type denoted by the message code.
func (d *Decoder) Decode() (interface{}, error) {

	// read from stream and extract code
	var data []byte
	//bs1 := binstat.EnterTime(binstat.BinNet + ":strm>1(cbor)iowriter2payload2envelope")
	err := d.dec.Decode(data)
	//binstat.LeaveVal(bs1, int64(len(data)))
	if err != nil || len(data) == 0 {
		return nil, fmt.Errorf("could not decode message; len(data)=%d: %w", len(data), err)
	}

	msgInterface, what, err := codec.InterfaceFromMessageCode(data[0])
	if err != nil {
		return nil, err
	}

	// unmarshal the payload
	//bs2 := binstat.EnterTimeVal(fmt.Sprintf("%s%s%s:%d", binstat.BinNet, ":strm>2(cbor)", what, code), int64(len(data))) // e.g. ~3net:strm>2(cbor)CodeEntityRequest:23
	err = defaultDecMode.Unmarshal(data[1:], msgInterface) // all but first byte
	//binstat.Leave(bs2)
	if err != nil {
		return nil, codec.NewMsgUnmarshalErr(data[0], what, err)
	}

	// TODO consider downsides of having this here:
	//  - performance?
	//  - surface area for unexpected errors from many StructureValidator impls?
	if validatable, ok := msgInterface.(model.StructureValidator); ok {
		if err := validatable.StructureValid(); err != nil {
			if model.IsStructureInvalidError(err) {
				return nil, codec.NewMsgUnmarshalErr(data[0], what, err)
			}
			return nil, fmt.Errorf("unexpected error validating structure of decoded message with type: %s: %w", what, err)
		}
	}

	return msgInterface, nil
}
