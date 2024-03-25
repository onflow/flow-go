package cbor

import (
	"github.com/fxamacker/cbor/v2"

	"github.com/onflow/flow-go/network/codec"
	_ "github.com/onflow/flow-go/utils/binstat"
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
	//bs1 := binstat.EnterTime(binstat.BinNet + ":strm>1(cbor)iowriter2payload2envelope")
	err := d.dec.Decode(&data)
	//binstat.LeaveVal(bs1, int64(len(data)))
	if err != nil {
		return nil, codec.NewInvalidEncodingErr(err)
	}

	msgCode, err := codec.MessageCodeFromPayload(data)
	if err != nil {
		return nil, err
	}

	msgInterface, what, err := codec.InterfaceFromMessageCode(msgCode)
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

	return msgInterface, nil
}
