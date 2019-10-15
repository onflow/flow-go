// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package xdr

import (
	"io"
)

// Encoder is an encoder to write serialized XDR to a writer.
type Encoder struct {
	w io.Writer
}

// Encode will convert the given message into binary XDR and write it to the
// underlying encoder.
func (e *Encoder) Encode(v interface{}) error {
	return encode(v, e.w)
}
