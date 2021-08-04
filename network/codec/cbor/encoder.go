// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package cbor

import (
	"fmt"

	"github.com/fxamacker/cbor/v2"

	_ "github.com/onflow/flow-go/utils/binstat"
)

// Encoder is an encoder to write serialized JSON to a writer.
type Encoder struct {
	enc *cbor.Encoder
}

// Encode will convert the given message into binary JSON and write it to the
// underlying encoder, followed by a new line.
func (e *Encoder) Encode(v interface{}) error {

	// encode the value
	data, code, err := v2envEncode(v, ":strm<1(cbor)")
	if err != nil {
		return fmt.Errorf("could not encode value: %w", err)
	}

	if (code <= CodeMin) || (code >= CodeMax) {
		return fmt.Errorf("invalid message envelope code: %d", code)
	}

	data = append(data, code)

	// write the envelope to network
	//bs := binstat.EnterTimeVal(binstat.BinNet+":strm<2(cbor)", int64(len(data)))
	err = e.enc.Encode(data)
	//binstat.Leave(bs)
	if err != nil {
		return fmt.Errorf("could not encode envelope: %w", err)
	}

	return nil
}
