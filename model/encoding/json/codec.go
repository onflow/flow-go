package json

import (
	"encoding/json"
	"io"

	"github.com/onflow/flow-go/model/encoding"
)

var _ encoding.Marshaler = (*Marshaler)(nil)

type Marshaler struct{}

func NewMarshaler() *Marshaler {
	return &Marshaler{}
}

func (m *Marshaler) Marshal(val any) ([]byte, error) {
	return json.Marshal(val)
}

func (m *Marshaler) Unmarshal(b []byte, val any) error {
	return json.Unmarshal(b, val)
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

type Codec struct{}

func (c *Codec) NewEncoder(w io.Writer) encoding.Encoder {
	return json.NewEncoder(w)
}

func (c *Codec) NewDecoder(r io.Reader) encoding.Decoder {
	return json.NewDecoder(r)
}
