package blocks

import (
	"encoding/binary"
	"fmt"

	gethCommon "github.com/onflow/go-ethereum/common"
)

const (
	heightEncodingSize    = 8
	timestampEncodingSize = 8
	randomEncodingSize    = 32
	metaEncodingSize      = heightEncodingSize +
		timestampEncodingSize +
		randomEncodingSize
)

// Meta holds meta data about a block
type Meta struct {
	Height    uint64
	Timestamp uint64
	Random    gethCommon.Hash
}

// NewBlockMeta constructs a new block meta
func NewMeta(
	height uint64,
	timestamp uint64,
	random gethCommon.Hash,
) *Meta {
	return &Meta{
		Height:    height,
		Timestamp: timestamp,
		Random:    random,
	}
}

// Encode encodes a meta
func (bm *Meta) Encode() []byte {
	// encode meta data
	buffer := make([]byte, metaEncodingSize)
	pos := 0

	// encode height
	binary.BigEndian.PutUint64(buffer[pos:], uint64(bm.Height))
	pos += heightEncodingSize

	// encode timestamp
	binary.BigEndian.PutUint64(buffer[pos:], uint64(bm.Timestamp))
	pos += timestampEncodingSize

	// encode random
	copy(buffer[pos:pos+randomEncodingSize], bm.Random[:])

	return buffer
}

// MetaFromEncoded constructs a Meta from encoded data
func MetaFromEncoded(data []byte) (*Meta, error) {
	// check the data size
	if len(data) < metaEncodingSize {
		return nil, fmt.Errorf("encoded input too short: %d < %d", len(data), metaEncodingSize)
	}

	bm := &Meta{}

	pos := 0
	// decode height
	bm.Height = binary.BigEndian.Uint64(data[pos:])
	pos += heightEncodingSize

	// decode timestamp
	bm.Timestamp = binary.BigEndian.Uint64(data[pos:])
	pos += timestampEncodingSize

	// decode random
	bm.Random = gethCommon.BytesToHash(data[pos:])

	return bm, nil
}
