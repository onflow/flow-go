package json

import (
	"encoding/json"
)

type Encoder struct{}

func NewEncoder() *Encoder {
	return &Encoder{}
}

func (e *Encoder) Encode(val interface{}) ([]byte, error) {
	return json.Marshal(val)
}

func (e *Encoder) Decode(b []byte, val interface{}) error {
	return json.Unmarshal(b, val)
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
