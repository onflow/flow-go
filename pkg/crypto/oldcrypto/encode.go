package oldcrypto

import (
	"bytes"
	"encoding/gob"
	"log"
	"time"
)

// EncodeAsBytes encodes a series of arbitrary data into bytes.
func EncodeAsBytes(data ...interface{}) []byte {
	gob.Register(time.Time{})
	gob.Register(Hash{})
	gob.Register([]Hash{})
	gob.Register(MockSignature{})

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	// Encode the data
	err := enc.Encode(data)

	if err != nil {
		log.Fatal("encode error:", err)
	}

	return buf.Bytes()
}
