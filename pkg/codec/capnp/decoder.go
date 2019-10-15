// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package capnp

import (
	"github.com/pkg/errors"
	capnp "zombiezen.com/go/capnproto2"
)

// Decoder decodes values from a stream in capnproto format.
type Decoder struct {
	dec *capnp.Decoder
}

// Decode will decode the next value from the stream.
func (d *Decoder) Decode() (interface{}, error) {

	// decode next message
	msg, err := d.dec.Decode()
	if err != nil {
		return nil, errors.Wrap(err, "could not read message")
	}

	// decode the actual data
	v, err := decode(msg)
	if err != nil {
		return nil, errors.Wrap(err, "could not decode message")
	}

	return v, nil
}
