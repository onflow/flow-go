package storage

import (
	"encoding/binary"
	"fmt"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
)

const (
	encHeightSize        = 2
	encHashSize          = hash.HashLen
	encPathSize          = ledger.PathLen
	encPayloadLengthSize = 4
)

func EncodePayload(path ledger.Path, payload *ledger.Payload, scratch []byte) ([]byte, error) {
	encPayloadSize := ledger.EncodedPayloadLengthWithoutPrefix(payload, flattener.PayloadEncodingVersion)

	encodedNodeSize := encPathSize +
		encPayloadLengthSize +
		encPayloadSize

	buf := scratch
	if len(scratch) < encodedNodeSize {
		buf = make([]byte, encPathSize+encPayloadLengthSize, encodedNodeSize)
	}

	pos := 0

	// encode path (32 bytes path)
	copy(buf[pos:], path[:])
	pos += encPathSize

	// encode payload (4 bytes Big Endian for encoded payload length and n bytes encoded payload)
	binary.BigEndian.PutUint32(buf[pos:], uint32(encPayloadSize))
	pos += encPayloadLengthSize

	// EncodeAndAppendPayloadWithoutPrefix appends encoded payload to the resliced buf.
	// Returned buf is resliced to include appended payload.
	buf = ledger.EncodeAndAppendPayloadWithoutPrefix(buf[:pos], payload, flattener.PayloadEncodingVersion)
	return buf, nil
}

func DecodePayload(encoded []byte) (ledger.Path, *ledger.Payload, error) {
	if len(encoded) < encPathSize+encPayloadLengthSize {
		return ledger.DummyPath, nil, fmt.Errorf("could not decode leaf node, not enough bytes: %v", len(encoded))
	}

	pos := 0

	// decode path
	path, err := ledger.ToPath(encoded[pos : pos+encPathSize])
	if err != nil {
		return ledger.DummyPath, nil, fmt.Errorf("could node decode path: %w", err)
	}
	pos += encPathSize

	// decode payload size
	expectedSize := binary.BigEndian.Uint32(encoded[pos : pos+encPayloadLengthSize])
	pos += encPayloadLengthSize

	// decode payload
	actualSize := uint32(len(encoded) - pos)
	if expectedSize != actualSize {
		return ledger.DummyPath, nil, fmt.Errorf("incorrect payload size, expect %v, actual %v", expectedSize, actualSize)
	}

	payload, err := ledger.DecodePayloadWithoutPrefix(encoded[pos:], false, flattener.PayloadEncodingVersion)
	if err != nil {
		return ledger.DummyPath, nil, fmt.Errorf("could not decode payload: %w", err)
	}

	return path, payload, nil
}
