// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package cbor

import (
	"fmt"

	"github.com/fxamacker/cbor/v2"

	_ "github.com/onflow/flow-go/utils/binstat"
)

// Decoder implements a stream decoder for CBOR.
type Decoder struct {
	dec *cbor.Decoder
}

// Decode will decode the next CBOR value from the stream.
func (d *Decoder) Decode() (interface{}, error) {

	// read from stream and extract code
	var data []byte
	//bs1 := binstat.EnterTime(binstat.BinNet + ":strm>1(cbor)iowriter2payload2envelope")
	err := d.dec.Decode(data)
	//binstat.LeaveVal(bs1, int64(len(data)))
	if err != nil || len(data) == 0 {
		return nil, fmt.Errorf("could not decode envelope; len(data)=%d: %w", len(data), err)
	}

	code := data[0] // only first byte

	what, v, err := envelopeCode2v(code)
	if err != nil {
		return nil, fmt.Errorf("could not determine interface from code: %w", err)
	}

	// unmarshal the payload
	//bs2 := binstat.EnterTimeVal(fmt.Sprintf("%s%s%s:%d", binstat.BinNet, ":strm>2(cbor)", what, code), int64(len(data))) // e.g. ~3net:strm>2(cbor)CodeEntityRequest:23
	err = cbor.Unmarshal(data[1:], v) // all but first byte
	//binstat.Leave(bs2)
	if err != nil {
		return nil, fmt.Errorf("could not decode CBOR payload with envelop code %d AKA %s: %w", code, what, err) // e.g. 2, "CodeBlockProposal", <CBOR error>
	}

	return v, nil
}
