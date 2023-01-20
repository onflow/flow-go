package storage

import (
	"testing"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeLeafNode(t *testing.T) {
	leafNode := leafNodeFixture()
	scratch := make([]byte, 1024*4)

	// encode the leaf node
	nodeHash, encoded, err := EncodeLeafNode(leafNode, scratch)
	require.NoError(t, err)

	// decode the leaf node
	decoded, err := DecodeLeafNode(nodeHash, encoded)
	require.NoError(t, err)

	// encode again the decoded leaf node
	fromDecodedHash, encodedFromDecoded, err := EncodeLeafNode(decoded, scratch)
	require.NoError(t, err)

	// encode again from the decoded leaf node should be the same as originally encoded
	require.Equal(t, nodeHash, fromDecodedHash)
	require.Equal(t, encoded, encodedFromDecoded)
}

func leafNodeFixture() *node.Node {
	height := 255
	path := testutils.PathByUint8(0)
	payload := testutils.LightPayload8('A', 'a')
	nodeHash := hash.Hash([32]byte{1, 2, 3})
	return node.NewNode(height, nil, nil, ledger.Path(path), payload, nodeHash)
}
