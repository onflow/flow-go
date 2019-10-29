// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package captain

import (
	"github.com/pkg/errors"
	capnp "zombiezen.com/go/capnproto2"
)

// Encoder encodes values to a a stream in capnproto format.
type Encoder struct {
	enc *capnp.Encoder
}

// Encoder will encode the value onto the stream.
func (e *Encoder) Encode(v interface{}) error {

	// encode the message
	msg, err := encode(v)
	if err != nil {
		return errors.Wrap(err, "could not serialize message")
	}

	// serialize the message
	err = e.enc.Encode(msg)
	if err != nil {
		return errors.Wrap(err, "could not write message")
	}

	return nil
}
