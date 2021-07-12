// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package cbor

import (
	"fmt"

	"github.com/fxamacker/cbor/v2"

	"github.com/onflow/flow-go/binstat"
)

// Decoder implements a stream decoder for JSON.
type Decoder struct {
	dec *cbor.Decoder
}

// Decode will decode the next JSON value from the stream.
func (d *Decoder) Decode() (interface{}, error) {

	// decode the next envelope
	var env Envelope
	p := binstat.EnterTime("~3net:strm>1", "")
	err := d.dec.Decode(&env)
	binstat.Leave(p)
	binstat.LeaveVal(p, int64(len(env.Data)))
	if err != nil {
		return nil, fmt.Errorf("could not decode envelope: %w", err)
	}

	// decode the embedded value
	v, err := env2vDecode(env, "~3net:strm>2")
	if err != nil {
		return nil, fmt.Errorf("could not decode value: %w", err)
	}

	return v, nil
}
