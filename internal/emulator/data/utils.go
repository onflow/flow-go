package data

import (
	"bytes"
	"encoding/gob"
	"log"
	"time"

	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

// EncodeAsBytes encodes a series of arbitrary data into bytes.
func EncodeAsBytes(data ...interface{}) []byte {
	gob.Register(time.Time{})
	gob.Register(crypto.Hash{})
	gob.Register([]crypto.Hash{})
	gob.Register(crypto.Address{})

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	// Encode the data
	err := enc.Encode(data)

	if err != nil {
		log.Fatal("encode error:", err)
	}

	return buf.Bytes()
}
