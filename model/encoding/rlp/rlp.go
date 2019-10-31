package rlp

import (
	"github.com/ethereum/go-ethereum/rlp"
)

type Encoder struct{}

func NewEncoder() *Encoder {
	return &Encoder{}
}

func (e *Encoder) Encode(val interface{}) ([]byte, error) {
	return rlp.EncodeToBytes(val)
}

func (e *Encoder) Decode(b []byte, val interface{}) error {
	return rlp.DecodeBytes(b, val)
}
