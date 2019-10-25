// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package capnp

import (
	"io"

	"github.com/pkg/errors"
	capnp "zombiezen.com/go/capnproto2"

	"github.com/dapperlabs/flow-go/pkg/codec"
)

// Codec implements a codec with capnproto serialization format.
type Codec struct {
}

// New creates a new capnp codec.
func NewCodec() *Codec {
	c := Codec{}
	return &c
}

// NewEncoder creates a new encoder using capnproto serialization.
func (c *Codec) NewEncoder(w io.Writer) codec.Encoder {
	e := Encoder{
		enc: capnp.NewEncoder(w),
	}
	return &e
}

// NewDecoder creates a new decoder using capnproto serialization.
func (c *Codec) NewDecoder(r io.Reader) codec.Decoder {
	d := Decoder{
		dec: capnp.NewDecoder(r),
	}
	return &d
}

// Encode will encode the given value to binary capnproto representation.
func (c *Codec) Encode(v interface{}) ([]byte, error) {

	// create capnp message
	msg, err := encode(v)
	if err != nil {
		return nil, errors.Wrap(err, "could not serialize message")
	}

	// get the bytes
	data, err := msg.Marshal()
	if err != nil {
		return nil, errors.Wrap(err, "could not write message")
	}

	return data, nil
}

// Decode will decode the value from the given binary capnproto representation.
func (c *Codec) Decode(data []byte) (interface{}, error) {

	// read the capnp message
	msg, err := capnp.Unmarshal(data)
	if err != nil {
		return nil, errors.Wrap(err, "could not read message")
	}

	// decode the message
	v, err := decode(msg)
	if err != nil {
		return nil, errors.Wrap(err, "could not deserialize message")
	}

	return v, nil
}
