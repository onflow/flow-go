package cbor

import (
	"fmt"
	"io"

	cbor "github.com/fxamacker/cbor/v2"

	"github.com/onflow/flow-go/model/encoding"
)

var _ encoding.Marshaler = (*Marshaler)(nil)

type Marshaler struct{}

func NewMarshaler() *Marshaler {
	return &Marshaler{}
}

// EncMode is the default EncMode to use when creating a new cbor Encoder
var EncMode = func() cbor.EncMode {
	// "For best performance, reuse EncMode and DecMode after creating them." [1]
	// [1] https://github.com/fxamacker/cbor
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

// UnsafeDecMode is a permissive mode for creating a new cbor Decoder.
//
// CAUTION: this encoding should only be used for encoding/decoding data within a node.
// If used for decoding data that is shared between nodes, it makes the recipient VULNERABLE
// to RESOURCE EXHAUSTION ATTACKS, where a byzantine sender could include garbage data in the
// encoding, which would not be noticed by the recipient because the garbage data is dropped
// at the decoding step - yet, it consumes the recipient's networking bandwidth.
var UnsafeDecMode, _ = cbor.DecOptions{}.DecMode()

// DefaultDecMode is the DecMode used for decoding messages over the network.
// It returns an error if the message contains any extra field not present in the
// target (struct we are unmarshalling into), which prevents some classes of resource exhaustion attacks.
var DefaultDecMode, _ = cbor.DecOptions{ExtraReturnErrors: cbor.ExtraDecErrorUnknownField}.DecMode()

func (m *Marshaler) Marshal(val any) ([]byte, error) {
	return EncMode.Marshal(val)
}

func (m *Marshaler) Unmarshal(b []byte, val any) error {
	return cbor.Unmarshal(b, val)
}

func (m *Marshaler) MustMarshal(val any) []byte {
	b, err := m.Marshal(val)
	if err != nil {
		panic(err)
	}

	return b
}

func (m *Marshaler) MustUnmarshal(b []byte, val any) {
	err := m.Unmarshal(b, val)
	if err != nil {
		panic(err)
	}
}

var _ encoding.Codec = (*Codec)(nil)

type Option func(*Codec)

// WithEncMode sets the EncMode to use when creating a new cbor Encoder.
func WithEncMode(encMode cbor.EncMode) Option {
	return func(c *Codec) {
		c.encMode = encMode
	}
}

// WithDecMode sets the DecMode to use when creating a new cbor Decoder.
func WithDecMode(decMode cbor.DecMode) Option {
	return func(c *Codec) {
		c.decMode = decMode
	}
}

type Codec struct {
	encMode cbor.EncMode
	decMode cbor.DecMode
}

// NewCodec returns a new cbor Codec with the provided EncMode and DecMode.
// If either is nil, the default cbor EncMode/DecMode will be used.
func NewCodec(opts ...Option) *Codec {
	c := &Codec{
		encMode: EncMode,
		decMode: UnsafeDecMode,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *Codec) NewEncoder(w io.Writer) encoding.Encoder {
	return c.encMode.NewEncoder(w)
}

func (c *Codec) NewDecoder(r io.Reader) encoding.Decoder {
	return c.decMode.NewDecoder(r)
}
