package state

import (
	"bytes"
	"encoding/gob"
	"log"
)

// Encode serializes a World State object into bytes.
func (ws *WorldState) Encode() []byte {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	err := enc.Encode(*ws)

	if err != nil {
		log.Fatal("encode error:", err)
	}

	return buf.Bytes()
}

// Decode takes a series of bytes that represents a World State
// object and decodes it into the respective struct type.
func Decode(b []byte) *WorldState {
	var buf bytes.Buffer
	buf.Write(b)
	dec := gob.NewDecoder(&buf)

	var ws WorldState
	err := dec.Decode(&ws)

	if err != nil {
		log.Fatal("decode error:", err)
	}

	return &ws
}
