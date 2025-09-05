package accountkeymetadata

import (
	"encoding/binary"
	"fmt"
)

const (
	lengthPrefixSize = 4
	runLengthSize    = 2
	digestSize       = 8
)

func parseWeightAndRevokedStatusFromKeyMetadataBytes(b []byte) (
	weightAndRevokedStatusBytes []byte,
	rest []byte,
	err error,
) {
	if len(b) == 0 {
		err = NewKeyMetadataMalfromedError("key metadata is empty")
		return
	}

	return parseNextLengthPrefixedData(b)
}

func parseStoredKeyMappingFromKeyMetadataBytes(b []byte) (
	startIndexForMapping uint32,
	mappingBytes []byte,
	rest []byte,
	err error,
) {
	if len(b) < storedKeyIndexSize {
		err = NewKeyMetadataMalfromedError(fmt.Sprintf("failed to parse start index for mapping: expect %d bytes, got %d bytes", storedKeyIndexSize, len(b)))
		return
	}

	// Get mapping start index
	startIndexForMapping = binary.BigEndian.Uint32(b[:storedKeyIndexSize])

	b = b[storedKeyIndexSize:]

	// Get mapping raw bytes
	mappingBytes, rest, err = parseNextLengthPrefixedData(b)
	return
}

func parseDigestsFromKeyMetadataBytes(b []byte) (
	startIndexForDigests uint32,
	digestBytes []byte,
	rest []byte,
	err error,
) {
	if len(b) < storedKeyIndexSize {
		err = NewKeyMetadataMalfromedError(fmt.Sprintf("failed to parse start index for digests: expect %d bytes, got %d bytes", storedKeyIndexSize, len(b)))
		return
	}

	// Get digest start index
	startIndexForDigests = binary.BigEndian.Uint32(b[:storedKeyIndexSize])

	b = b[storedKeyIndexSize:]

	// Get digests raw bytes
	digestBytes, rest, err = parseNextLengthPrefixedData(b)
	return
}

func parseNextLengthPrefixedData(b []byte) (next []byte, rest []byte, err error) {
	if len(b) < lengthPrefixSize {
		return nil, nil, NewKeyMetadataMalfromedError(fmt.Sprintf("expect length prefix (4 bytes), got %d bytes", len(b)))
	}

	length := binary.BigEndian.Uint32(b[:lengthPrefixSize])

	// NOTE: here, int is always int64 (never int32) because this software can cannot run on 32-bit platforms,
	// so it is safe to cast length (uint32) to int (which is int64 on 64-bit platforms).
	if len(b) < lengthPrefixSize+int(length) {
		return nil, nil, NewKeyMetadataMalfromedError(fmt.Sprintf("expect %d bytes for next data, got %d bytes", lengthPrefixSize+length, len(b)))
	}

	b = b[lengthPrefixSize:]
	return b[:length], b[length:], nil
}
