// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package cbor

import (
	"fmt"
	"io"

	"github.com/fxamacker/cbor/v2"

	"github.com/onflow/flow-go/network"
	_ "github.com/onflow/flow-go/utils/binstat"
)

// Codec represents a JSON codec for our network.
type Codec struct {
}

// NewCodec creates a new JSON codec.
func NewCodec() *Codec {
	c := &Codec{}
	return c
}

// NewEncoder creates a new JSON encoder with the given underlying writer.
func (c *Codec) NewEncoder(w io.Writer) network.Encoder {
	enc := cbor.NewEncoder(w)
	return &Encoder{enc: enc}
}

// NewDecoder creates a new JSON decoder with the given underlying reader.
func (c *Codec) NewDecoder(r io.Reader) network.Decoder {
	dec := cbor.NewDecoder(r)
	return &Decoder{dec: dec}
}

// Encode will encode the givene entity and return the bytes.
func (c *Codec) Encode(v interface{}) ([]byte, error) {

	// encode the value
	data, code, err := v2envEncode(v, ":wire<1(cbor)")
	if err != nil {
		return nil, fmt.Errorf("could not encode envelope: %w", err)
	}

	// encode the envelope
	//bs := binstat.EnterTime(binstat.BinNet + ":wire<2(cbor)envelope2payload")
	data = append(data, code)
	//binstat.LeaveVal(bs, int64(len(data)))

	return data, nil
}

// Decode will attempt to decode the given entity from bytes.
func (c *Codec) Decode(data []byte) (interface{}, error) {

	// decode the envelope
	//bs := binstat.EnterTime(binstat.BinNet + ":wire>3(cbor)payload2envelope")
	code := data[len(data)-1] // only last byte
	//binstat.LeaveVal(bs, int64(len(data)))

	// decode the value
	v, err := env2vDecode(data[:len(data)-1], code, ":wire>4(cbor)") // all but last byte
	if err != nil {
		return nil, fmt.Errorf("could not decode value: %w", err)
	}

	return v, nil
}
