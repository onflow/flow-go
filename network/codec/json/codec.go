// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package json

import (
	"encoding/json"
	"fmt"
	"io"

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
	enc := json.NewEncoder(w)
	return &Encoder{enc: enc}
}

// NewDecoder creates a new JSON decoder with the given underlying reader.
func (c *Codec) NewDecoder(r io.Reader) network.Decoder {
	dec := json.NewDecoder(r)
	return &Decoder{dec: dec}
}

// Encode will encode the givene entity and return the bytes.
func (c *Codec) Encode(v interface{}) ([]byte, error) {

	// encode the value
	env, err := v2envEncode(v, ":wire<1(json)")
	if err != nil {
		return nil, fmt.Errorf("could not encode envelope: %w", err)
	}

	// TODO: consider eliminating envelope / double .Marshal as implemented in sibling codec CBOR using append()?

	// encode the envelope
	//bs := binstat.EnterTime(binstat.BinNet + ":wire<2(json)envelope2payload")
	data, err := json.Marshal(env)
	//binstat.LeaveVal(bs, int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("could not encode value: %w", err)
	}

	return data, nil
}

// Decode will attempt to decode the given entity from bytes.
func (c *Codec) Decode(data []byte) (interface{}, error) {

	// decode the envelope
	var env Envelope
	//bs := binstat.EnterTime(binstat.BinNet + ":wire>3(json)payload2envelope")
	err := json.Unmarshal(data, &env)
	//binstat.LeaveVal(bs, int64(len(env.Data)))
	if err != nil {
		return nil, fmt.Errorf("could not decode envelope: %w", err)
	}

	// decode the value
	v, err := env2vDecode(env, ":wire>4(json)")
	if err != nil {
		return nil, fmt.Errorf("could not decode value: %w", err)
	}

	return v, nil
}
