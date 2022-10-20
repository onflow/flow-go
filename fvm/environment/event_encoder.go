package environment

import (
	"bytes"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
)

type EventEncoder interface {
	Encode(event cadence.Event) ([]byte, error)
}

type CadenceEventEncoder struct {
	buffer  *bytes.Buffer
	encoder *jsoncdc.Encoder
}

func NewCadenceEventEncoder() *CadenceEventEncoder {
	var buf bytes.Buffer
	return &CadenceEventEncoder{
		buffer:  &buf,
		encoder: jsoncdc.NewEncoder(&buf),
	}
}

func (e *CadenceEventEncoder) Encode(event cadence.Event) ([]byte, error) {
	e.buffer.Reset()

	err := e.encoder.Encode(event)
	if err != nil {
		return nil, err
	}
	b := e.buffer.Bytes()
	payload := make([]byte, len(b))
	copy(payload, b)

	return payload, nil
}
