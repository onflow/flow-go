package environment

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
)

type EventEncoder interface {
	Encode(event cadence.Event) ([]byte, error)
}

type CadenceEventEncoder struct{}

func NewCadenceEventEncoder() *CadenceEventEncoder {
	return &CadenceEventEncoder{}
}

func (e *CadenceEventEncoder) Encode(event cadence.Event) ([]byte, error) {
	return ccf.Encode(event)
}
