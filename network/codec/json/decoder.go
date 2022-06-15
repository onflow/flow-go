// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package json

import (
	"encoding/json"
	"fmt"

	_ "github.com/onflow/flow-go/utils/binstat"
)

// Decoder implements a stream decoder for JSON.
type Decoder struct {
	dec *json.Decoder
}

// Decode will decode the next JSON value from the stream.
func (d *Decoder) Decode() (interface{}, error) {

	// decode the next envelope
	var env Envelope
	//bs := binstat.EnterTime(binstat.BinNet + ":strm>1(json)")
	err := d.dec.Decode(&env)
	//binstat.LeaveVal(bs, int64(len(env.Data)))
	if err != nil {
		return nil, fmt.Errorf("could not decode envelope: %w", err)
	}

	// decode the embedded value
	v, err := env2vDecode(env, ":strm>2(json)")
	if err != nil {
		return nil, fmt.Errorf("could not decode value: %w", err)
	}

	return v, nil
}
