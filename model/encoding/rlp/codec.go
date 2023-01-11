package rlp

import (
	"io"

	"github.com/ethereum/go-ethereum/rlp"

	"github.com/onflow/flow-go/model/encoding"
)

var _ encoding.Marshaler = (*Marshaler)(nil)

type Marshaler struct{}

func NewMarshaler() *Marshaler {
	return &Marshaler{}
}

func (m *Marshaler) Marshal(val interface{}) ([]byte, error) {
	return rlp.EncodeToBytes(val)
}

func (m *Marshaler) Unmarshal(b []byte, val interface{}) error {
	return rlp.DecodeBytes(b, val)
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
	return &Encoder{w}
}

func (c *Codec) NewDecoder(r io.Reader) encoding.Decoder {
	return &Decoder{r}
}

type Encoder struct {
	w io.Writer
}

func (e *Encoder) Encode(v interface{}) error {
	return rlp.Encode(e.w, v)
}

type Decoder struct {
	r io.Reader
}

func (e *Decoder) Decode(v interface{}) error {
	return rlp.Decode(e.r, v)
}
