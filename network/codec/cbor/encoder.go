// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package cbor

import (
	"fmt"

	"github.com/fxamacker/cbor/v2"

	"github.com/onflow/flow-go/utils/binstat"
)

// Encoder is an encoder to write serialized JSON to a writer.
type Encoder struct {
	enc *cbor.Encoder
}

// Encode will convert the given message into binary JSON and write it to the
// underlying encoder, followed by a new line.
func (e *Encoder) Encode(v interface{}) error {

	// encode the value
	data, code, err := v2envEncode(v, "~3net:strm<1")
	if err != nil {
		return fmt.Errorf("could not encode value: %w", err)
	}

	if (code <= CodeMin) || (code >= CodeMax) {
		return fmt.Errorf("invalid message envelope code: %d", code)
	}

	data = append(data, code)

	// write the envelope to network
	bs := binstat.EnterTimeVal("~3net:strm<2", int64(len(data)))
	bs.Run(func() {
		err = e.enc.Encode(data)
	})
	bs.Leave()
	if err != nil {
		return fmt.Errorf("could not encode envelope: %w", err)
	}

	return nil
}
