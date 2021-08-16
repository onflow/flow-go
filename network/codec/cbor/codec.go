// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package cbor

import (
	"bytes"
	"fmt"
	"io"

	"github.com/fxamacker/cbor/v2"

	cborcodec "github.com/onflow/flow-go/model/encoding/cbor"
	"github.com/onflow/flow-go/network"
	_ "github.com/onflow/flow-go/utils/binstat"
)

// Codec represents a CBOR codec for our network.
type Codec struct {
}

// NewCodec creates a new CBOR codec.
func NewCodec() *Codec {
	c := &Codec{}
	return c
}

// NewEncoder creates a new CBOR encoder with the given underlying writer.
func (c *Codec) NewEncoder(w io.Writer) network.Encoder {
	enc := cborcodec.EncMode.NewEncoder(w)
	return &Encoder{enc: enc}
}

// NewDecoder creates a new CBOR decoder with the given underlying reader.
func (c *Codec) NewDecoder(r io.Reader) network.Decoder {
	dec := cbor.NewDecoder(r)
	return &Decoder{dec: dec}
}

// Given a Golang interface 'v', return a []byte 'envelope'.
// Return an error if packing the envelope fails.
// NOTE: 'v' is the network message payload in unserialized form.
// NOTE: 'code' is the message type.
// NOTE: 'what' is the 'code' name for debugging / instrumentation.
// NOTE: 'envelope' contains 'code' & serialized / encoded 'v'.
// i.e.  1st byte is 'code' and remaining bytes are CBOR encoded 'v'.
func (c *Codec) Encode(v interface{}) ([]byte, error) {

	// encode the value
	what, code, err := v2envelopeCode(v)
	if err != nil {
		return nil, fmt.Errorf("could not determine envelope code: %w", err)
	}

	// NOTE: benchmarking shows that prepending the code and then using
	//       .NewEncoder() to .Encode() is the fastest.

	// encode / append the envelope code
	//bs1 := binstat.EnterTime(binstat.BinNet + ":wire<1(cbor)envelope2payload")
	var data bytes.Buffer
	data.WriteByte(code)
	//binstat.LeaveVal(bs1, int64(data.Len()))

	// encode the payload
	//bs2 := binstat.EnterTime(fmt.Sprintf("%s%s%s:%d", binstat.BinNet, ":wire<2(cbor)", what, code)) // e.g. ~3net::wire<1(cbor)CodeEntityRequest:23
	encoder := cborcodec.EncMode.NewEncoder(&data)
	err = encoder.Encode(v)
	//binstat.LeaveVal(bs2, int64(data.Len()))
	if err != nil {
		return nil, fmt.Errorf("could not encode CBOR payload with envelope code %d AKA %s: %w", code, what, err) // e.g. 2, "CodeBlockProposal", <CBOR error>
	}

	dataBytes := data.Bytes()

	return dataBytes, nil
}

// Given a []byte 'envelope', eturn a Golang interface 'v'.
// Return an error if unpacking the envelope fails.
// NOTE: 'v' is the network message payload in unserialized form.
// NOTE: 'code' is the message type.
// NOTE: 'what' is the 'code' name for debugging / instrumentation.
// NOTE: 'envelope' contains 'code' & serialized / encoded 'v'.
// i.e.  1st byte is 'code' and remaining bytes are CBOR encoded 'v'.
func (c *Codec) Decode(data []byte) (interface{}, error) {

	// decode the envelope
	//bs1 := binstat.EnterTime(binstat.BinNet + ":wire>3(cbor)payload2envelope")
	code := data[0] // only first byte
	//binstat.LeaveVal(bs1, int64(len(data)))

	what, v, err := envelopeCode2v(code)
	if err != nil {
		return nil, fmt.Errorf("could not determine interface from code: %w", err)
	}

	// unmarshal the payload
	//bs2 := binstat.EnterTimeVal(fmt.Sprintf("%s%s%s:%d", binstat.BinNet, ":wire>4(cbor)", what, code), int64(len(data))) // e.g. ~3net:wire>4(cbor)CodeEntityRequest:23
	err = cbor.Unmarshal(data[1:], v) // all but first byte
	//binstat.Leave(bs2)
	if err != nil {
		return nil, fmt.Errorf("could not decode CBOR payload with envelope code %d AKA %s: %w", code, what, err) // e.g. 2, "CodeBlockProposal", <CBOR error>
	}

	return v, nil
}
