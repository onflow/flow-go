package environment

import (
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
)

type EventEncoder interface {
	Encode(event cadence.Event) ([]byte, error)
}

type CadenceEventEncoder struct{}

func NewCadenceEventEncoder() *CadenceEventEncoder {
	return &CadenceEventEncoder{}
}

func (e *CadenceEventEncoder) Encode(event cadence.Event) ([]byte, error) {
	return jsoncdc.Encode(event)
}
