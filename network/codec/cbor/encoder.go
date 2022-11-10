// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package cbor

import (
	"bytes"
	"fmt"

	"github.com/fxamacker/cbor/v2"

	cborcodec "github.com/onflow/flow-go/model/encoding/cbor"
	"github.com/onflow/flow-go/network/codec"
)

// Encoder is an encoder to write serialized CBOR to a writer.
type Encoder struct {
	enc *cbor.Encoder
}

// Encode will convert the given message into CBOR and write it to the
// underlying encoder, followed by a new line.
func (e *Encoder) Encode(v interface{}) error {
	// encode the value
	code, what, err := codec.MessageCodeFromInterface(v)
	if err != nil {
		return fmt.Errorf("could not determine envelope code string: %w", err)
	}

	// encode the payload
	var data bytes.Buffer
	_ = data.WriteByte(code)
	encoder := cborcodec.EncMode.NewEncoder(&data)
	err = encoder.Encode(v)
	if err != nil {
		return fmt.Errorf("could not encode cbor payload with message code %d aka %s: %w", code, what, err) // e.g. 2, "CodeBlockProposal", <CBOR error>
	}

	// encode / append the envelope code and write to stream
	dataBytes := data.Bytes()
	err = e.enc.Encode(dataBytes)
	if err != nil {
		return fmt.Errorf("could not encode to stream: %w", err)
	}

	return nil
}
