package common

import (
	"encoding/binary"

	"github.com/dapperlabs/flow-go/ledger"
)

// OneBytePath returns a path (1 byte) given a uint8
func OneBytePath(inp uint8) ledger.Path {
	return ledger.Path([]byte{inp})
}

// TwoBytesPath returns a path (2 bytes) given a uint16
func TwoBytesPath(inp uint16) ledger.Path {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, inp)
	return ledger.Path(b)
}

func LightPayload(key uint16, value uint16) *ledger.Payload {
	k := ledger.Key{KeyParts: []ledger.KeyPart{ledger.KeyPart{Type: 0, Value: Uint2binary(key)}}}
	v := ledger.Value(Uint2binary(value))
	return &ledger.Payload{Key: k, Value: v}
}

func LightPayload8(key uint8, value uint8) *ledger.Payload {
	k := ledger.Key{KeyParts: []ledger.KeyPart{ledger.KeyPart{Type: 0, Value: []byte{key}}}}
	v := ledger.Value([]byte{value})
	return &ledger.Payload{Key: k, Value: v}
}
