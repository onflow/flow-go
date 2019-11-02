// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package json

import (
	"encoding/json"

	"github.com/pkg/errors"
)

// Decoder implements a stream decoder for JSON.
type Decoder struct {
	dec *json.Decoder
}

// Decode will decode the next JSON value from the stream.
func (d *Decoder) Decode() (interface{}, error) {

	// decode the next envelope
	var env Envelope
	err := d.dec.Decode(&env)
	if err != nil {
		return nil, errors.Wrap(err, "could not decode envelope")
	}

	// decode the embedded value
	v, err := decode(env)
	if err != nil {
		return nil, errors.Wrap(err, "could not decode value")
	}

	return v, nil
}
