package cbor

import (
	"fmt"

	"github.com/fxamacker/cbor/v2"
)

type Encoder struct{}

func NewEncoder() *Encoder {
	return &Encoder{}
}

// "For best performance, reuse EncMode and DecMode after creating them." [1]
// [1] https://github.com/fxamacker/cbor
var EncMode = func() cbor.EncMode {
	options := cbor.CoreDetEncOptions() // CBOR deterministic options
	// default: "2021-07-06 21:20:00 +0000 UTC" <- unwanted
	// option : "2021-07-06 21:20:00.820603 +0000 UTC" <- wanted
	options.Time = cbor.TimeRFC3339Nano // option needed for wanted time format
	encMode, err := options.EncMode()
	if err != nil {
		panic(fmt.Errorf("could not extract encoding mode: %w", err))
	}
	return encMode
}()

func (e *Encoder) Encode(val interface{}) ([]byte, error) {
	return EncMode.Marshal(val)
}

func (e *Encoder) Decode(b []byte, val interface{}) error {
	return cbor.Unmarshal(b, val)
}

func (e *Encoder) MustEncode(val interface{}) []byte {
	b, err := e.Encode(val)
	if err != nil {
		panic(err)
	}

	return b
}

func (e *Encoder) MustDecode(b []byte, val interface{}) {
	err := e.Decode(b, val)
	if err != nil {
		panic(err)
	}
}
