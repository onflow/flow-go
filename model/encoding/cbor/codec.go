package cbor

import (
	"fmt"
	"io"

	"github.com/fxamacker/cbor/v2"

	"github.com/onflow/flow-go/model/encoding"
)

var _ encoding.Marshaler = (*Marshaler)(nil)

type Marshaler struct{}

func NewMarshaler() *Marshaler {
	return &Marshaler{}
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

func (m *Marshaler) Marshal(val interface{}) ([]byte, error) {
	return EncMode.Marshal(val)
}

func (m *Marshaler) Unmarshal(b []byte, val interface{}) error {
	return cbor.Unmarshal(b, val)
}

func (m *Marshaler) MustMarshal(val interface{}) []byte {
	b, err := m.Marshal(val)
	if err != nil {
		panic(err)
	}

	return b
}

func (m *Marshaler) MustUnmarshal(b []byte, val interface{}) {
	err := m.Unmarshal(b, val)
	if err != nil {
		panic(err)
	}
}

var _ encoding.Codec = (*Codec)(nil)

type Codec struct{}

func (c *Codec) NewEncoder(w io.Writer) encoding.Encoder {
	return EncMode.NewEncoder(w)
}

func (c *Codec) NewDecoder(r io.Reader) encoding.Decoder {
	return cbor.NewDecoder(r)
}
