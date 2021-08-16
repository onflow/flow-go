// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package cbor

import (
	"bytes"
	"fmt"

	"github.com/fxamacker/cbor/v2"

	cborcodec "github.com/onflow/flow-go/model/encoding/cbor"
	_ "github.com/onflow/flow-go/utils/binstat"
)

// Encoder is an encoder to write serialized CBOR to a writer.
type Encoder struct {
	enc *cbor.Encoder
}

// Encode will convert the given message into CBOR and write it to the
// underlying encoder, followed by a new line.
func (e *Encoder) Encode(v interface{}) error {

	// encode the value
	what, code, err := v2envelopeCode(v)
	if err != nil {
		return fmt.Errorf("could not determine envelope code: %w", err)
	}

	// encode the payload
	//bs1 := binstat.EnterTime(fmt.Sprintf("%s%s%s:%d", binstat.BinNet, ":strm<1(cbor)", what, code)) // e.g. ~3net::strm<1(cbor)CodeEntityRequest:23
	var data bytes.Buffer
	data.WriteByte(code)
	encoder := cborcodec.EncMode.NewEncoder(&data)
	err = encoder.Encode(v)
	//binstat.LeaveVal(bs1, int64(data.Len()))
	if err != nil {
		return fmt.Errorf("could not encode CBOR payload with envelope code %d AKA %s: %w", code, what, err) // e.g. 2, "CodeBlockProposal", <CBOR error>
	}

	// encode / append the envelope code and write to stream
	//bs2 := binstat.EnterTime(binstat.BinNet+":strm<2(cbor)envelope2payload2iowriter")
	dataBytes := data.Bytes()
	err = e.enc.Encode(dataBytes)
	//binstat.LeaveVal(bs2, int64(data.Len()))
	if err != nil {
		return fmt.Errorf("could not encode to stream: %w", err)
	}

	return nil
}
