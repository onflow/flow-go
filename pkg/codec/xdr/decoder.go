// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package xdr

import (
	"io"
)

// Decoder implements a stream decoder for JSON.
type Decoder struct {
	r io.Reader
}

// Decode will decode the next JSON value from the stream.
func (d *Decoder) Decode() (interface{}, error) {
	return decode(d.r)
}
