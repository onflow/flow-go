// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package cbor

import (
	"fmt"

	"github.com/fxamacker/cbor/v2"

	_ "github.com/onflow/flow-go/utils/binstat"
)

// Decoder implements a stream decoder for JSON.
type Decoder struct {
	dec *cbor.Decoder
}

// Decode will decode the next JSON value from the stream.
func (d *Decoder) Decode() (interface{}, error) {

	// decode the next envelope
	var data []byte
	//bs := binstat.EnterTime(binstat.BinNet + ":strm>1(cbor)")
	err := d.dec.Decode(data)
	//binstat.LeaveVal(bs, int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("could not decode envelope: %w", err)
	}

	code := data[len(data)-1] // only last byte

	// decode the embedded value
	v, err := env2vDecode(data[:len(data)-1], code, ":strm>2(cbor)") // all but last byte
	if err != nil {
		return nil, fmt.Errorf("could not decode value: %w", err)
	}

	return v, nil
}
