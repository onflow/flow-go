package indexes

import (
	"encoding/binary"
	"fmt"
)

// encodedUint64 encodes uint64 for storing as a pebble payload
func encodedUint64(height uint64) []byte {
	payload := make([]byte, 0, 8)
	return binary.BigEndian.AppendUint64(payload, height)
}

// decodeUint64 decodes uint64 from a byte slice.
func decodeUint64(value []byte) (uint64, error) {
	if len(value) != 8 {
		return 0, fmt.Errorf("invalid value length: expected %d, got %d", 8, len(value))
	}
	return binary.BigEndian.Uint64(value), nil
}
