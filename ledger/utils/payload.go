package utils

import (
	"math/rand"

	"github.com/dapperlabs/flow-go/ledger"
)

const (
	minByteSize = 2
	maxByteSize = 8
)

// Payload implements a basic version of storage payload
type Payload struct {
	key   []byte
	value []byte
}

// TypeVersion returns the version of this payload encoding
func (p *Payload) TypeVersion() uint8 {
	return uint8(0)
}

// ID returns a unique id for this
func (p *Payload) ID() ledger.PayloadID {
	return ledger.PayloadID(p.key)
}

// Encode encodes the the payload object
func (p *Payload) Encode() []byte {
	// TODO include key in the encode
	return p.value
}

// NewPayload returns a basic payload
func NewPayload(key []byte, value []byte) ledger.Payload {
	return &Payload{key: key, value: value}
}

// RandomPayload returns a random payload
func RandomPayload() ledger.Payload {
	keyByteSize := minByteSize + rand.Intn(maxByteSize-minByteSize)
	key := make([]byte, keyByteSize)
	rand.Read(key)
	valueByteSize := minByteSize + rand.Intn(maxByteSize-minByteSize)
	value := make([]byte, valueByteSize)
	rand.Read(value)
	return &Payload{key: key, value: value}
}
