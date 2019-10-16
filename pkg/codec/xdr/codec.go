// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package xdr

import (
	"bytes"
	"io"

	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/pkg/codec"
)

// Codec is a encoder/decoder using the XDR serialization format.
type Codec struct {
}

// NewCodec creates a new XDR codec.
func NewCodec() *Codec {
	c := Codec{}
	return &c
}

// NewEncoder creates a new XDR encoder that encodes events directly onto the
// provided writer.
func (c *Codec) NewEncoder(w io.Writer) codec.Encoder {
	return &Encoder{w: w}
}

// NewDecoder creates a new XDR decoder that decodes events directly from the
// provided reader.
func (c *Codec) NewDecoder(r io.Reader) codec.Decoder {
	return &Decoder{r: r}
}

// Encode encodes the event to bytes using XDR.
func (c *Codec) Encode(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	err := encode(v, &buf)
	if err != nil {
		return nil, errors.Wrap(err, "could not encode to buffer")
	}
	return buf.Bytes(), nil
}

// Decode decodes the event from bytes using XDR.
func (c *Codec) Decode(data []byte) (interface{}, error) {
	buf := bytes.NewBuffer(data)
	v, err := decode(buf)
	if err != nil {
		return nil, errors.Wrap(err, "could not decode from buffer")
	}
	return v, nil
}
