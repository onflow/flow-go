package accountkeymetadata

import (
	"encoding/binary"

	"github.com/onflow/flow-go/fvm/errors"
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
		err = errors.NewKeyMetadataEmptyError("failed to parse weight and revoked status")
		return
	}

	return parseNextLengthPrefixedData(b)
}

// parseStoredKeyMappingFromKeyMetadataBytes parses b and returns:
// start index for mapping, raw mapping bytes, trailing bytes, and error (if any).
// NOTE: b is expected to start with encoded start index for mapping.
func parseStoredKeyMappingFromKeyMetadataBytes(b []byte) (
	startIndexForMapping uint32,
	mappingBytes []byte,
	rest []byte,
	err error,
) {
	if len(b) < storedKeyIndexSize {
		err = errors.NewKeyMetadataTooShortError(
			"failed to parse start key index for mappings",
			storedKeyIndexSize,
			len(b),
		)
		return
	}

	// Get mapping start index
	startIndexForMapping = binary.BigEndian.Uint32(b[:storedKeyIndexSize])

	b = b[storedKeyIndexSize:]

	// Get mapping raw bytes
	mappingBytes, rest, err = parseNextLengthPrefixedData(b)
	return
}

// parseDigestsFromKeyMetadataBytes parses b and returns:
// start index for digests, raw digest bytes, trailing bytes, and error (if any).
// NOTE: b is expected to start with encoded start index for digests.
func parseDigestsFromKeyMetadataBytes(b []byte) (
	startIndexForDigests uint32,
	digestBytes []byte,
	rest []byte,
	err error,
) {
	if len(b) < storedKeyIndexSize {
		err = errors.NewKeyMetadataTooShortError(
			"failed to parse start key index for digests",
			storedKeyIndexSize,
			len(b),
		)
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
		return nil, nil,
			errors.NewKeyMetadataTooShortError(
				"failed to parse prefixed data",
				lengthPrefixSize,
				len(b),
			)
	}

	length := binary.BigEndian.Uint32(b[:lengthPrefixSize])

	// NOTE: here, int is always int64 (never int32) because this software can cannot run on 32-bit platforms,
	// so it is safe to cast length (uint32) to int (which is int64 on 64-bit platforms).
	if len(b) < lengthPrefixSize+int(length) {
		return nil, nil,
			errors.NewKeyMetadataTooShortError(
				"failed to parse length prefixed data",
				lengthPrefixSize+int(length),
				len(b),
			)
	}

	b = b[lengthPrefixSize:]
	return b[:length], b[length:], nil
}
