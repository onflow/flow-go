// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package json

import (
	"encoding/json"

	"github.com/pkg/errors"
)

// Encoder is an encoder to write serialized JSON to a writer.
type Encoder struct {
	enc *json.Encoder
}

// Encode will convert the given message into binary JSON and write it to the
// underlying encoder, followed by a new line.
func (e *Encoder) Encode(v interface{}) error {

	// encode the value
	env, err := encode(v)
	if err != nil {
		return errors.Wrap(err, "could not encode value")
	}

	// write the envelope to network
	err = e.enc.Encode(env)
	if err != nil {
		return errors.Wrap(err, "could not encode envelope")
	}

	return nil
}
