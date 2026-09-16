package payloadless

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger/common/hash"
)

// TestReadLeafHashFromReaderAbsent covers the leafHashAbsent branch: only the
// flag byte is encoded and no leaf hash follows. This is the on-disk encoding
// v7 uses for a payload hash that has never been allocated, so the reader must
// consume exactly the flag byte and report a nil hash.
func TestReadLeafHashFromReaderAbsent(t *testing.T) {
	reader := bytes.NewReader([]byte{leafHashAbsent})

	leafHash, err := readLeafHashFromReader(reader, nil)
	require.NoError(t, err)
	require.Nil(t, leafHash)
	require.Zero(t, reader.Len(), "absent flag must consume exactly one byte")
}

// TestReadLeafHashFromReaderPresent covers the leafHashPresent branch: the flag
// is followed by a 32-byte hash, which must be decoded verbatim.
func TestReadLeafHashFromReaderPresent(t *testing.T) {
	var expected hash.Hash
	for i := range expected {
		expected[i] = byte(i + 1)
	}
	reader := bytes.NewReader(append([]byte{leafHashPresent}, expected[:]...))

	leafHash, err := readLeafHashFromReader(reader, nil)
	require.NoError(t, err)
	require.NotNil(t, leafHash)
	require.Equal(t, expected, *leafHash)
	require.Zero(t, reader.Len())
}

// TestReadLeafHashFromReaderInvalidFlag covers the default branch: any flag
// other than leafHashAbsent/leafHashPresent must be rejected rather than
// silently mis-decoded.
func TestReadLeafHashFromReaderInvalidFlag(t *testing.T) {
	for _, flag := range []byte{2, 0xff} {
		reader := bytes.NewReader([]byte{flag})
		leafHash, err := readLeafHashFromReader(reader, nil)
		require.Error(t, err, "flag %d must be rejected", flag)
		require.Nil(t, leafHash)
	}
}

// TestReadLeafHashFromReaderTruncatedPresent covers a present flag whose hash
// bytes are cut short.
func TestReadLeafHashFromReaderTruncatedPresent(t *testing.T) {
	reader := bytes.NewReader(append([]byte{leafHashPresent}, make([]byte, encHashSize-1)...))

	leafHash, err := readLeafHashFromReader(reader, nil)
	require.Error(t, err)
	require.Nil(t, leafHash)
}
