package state

import (
	"bytes"
	"encoding/gob"
	"log"
)

func (ws *WorldState) Encode() []byte {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	// Encode the data
	err := enc.Encode(*ws)

	if err != nil {
		log.Fatal("encode error:", err)
	}

	return buf.Bytes()
}

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
