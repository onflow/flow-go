package storage

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/testutils"
)

func TestEncodeDecodePayload(t *testing.T) {
	path := testutils.PathByUint8(0)
	scratch := make([]byte, 1024*4)
	payload := testutils.LightPayload8('A', 'a')

	// encode the payload
	encoded, err := EncodePayload(path, payload, scratch)
	require.NoError(t, err, "can not encode payload")

	// decode the payload
	decodedPath, decodedPayload, err := DecodePayload(encoded)
	require.NoError(t, err)

	// decoded should be the same as original
	require.Equal(t, path, decodedPath)
	require.True(t, payload.Equals(decodedPayload))

	// encode again the decoded leaf node
	encodedAgain, err := EncodePayload(decodedPath, decodedPayload, scratch)
	require.NoError(t, err)

	require.Equal(t, encoded, encodedAgain)
}

func TestEncodeDecodeEmptyPayload(t *testing.T) {
	path := testutils.PathByUint8(0)
	scratch := make([]byte, 1024*4)
	payload := ledger.EmptyPayload()

	// encode the payload
	encoded, err := EncodePayload(path, payload, scratch)
	require.NoError(t, err, "can not encode payload")

	// decode the payload
	decodedPath, decodedPayload, err := DecodePayload(encoded)
	require.NoError(t, err)

	// decoded should be the same as original
	require.Equal(t, path, decodedPath)
	require.True(t, payload.Equals(decodedPayload))
	require.True(t, decodedPayload.IsEmpty())
}
