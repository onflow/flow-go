package payloadless

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/testutils"
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

// noChildNodes is the getNode callback for reading leaf nodes, which never reference children.
func noChildNodes(t *testing.T) func(nodeIndex uint64) (*Node, error) {
	return func(nodeIndex uint64) (*Node, error) {
		require.FailNow(t, "leaf node must not resolve child nodes", "index %d", nodeIndex)
		return nil, nil
	}
}

// TestEncodeDecodeLeafNodeWithLeafHash covers the `leafHashPresent` encoding: an allocated
// register's leaf keeps its leaf hash across a round trip, and the encoding is 100 bytes.
func TestEncodeDecodeLeafNodeWithLeafHash(t *testing.T) {
	path := testutils.PathByUint8(7)
	leaf := NewLeaf(path, []byte("register value"), 256)
	require.NotNil(t, leaf.LeafHash(), "sanity check: an allocated register leaf has a leaf hash")

	scratch := make([]byte, 1024)
	encoded := EncodeNode(leaf, 0, 0, scratch)

	// node type (1) + height (2) + hash (32) + path (32) + leaf hash flag (1) + leaf hash (32)
	require.Len(t, encoded, 100)
	require.Equal(t, leafHashPresent, encoded[encNodeTypeSize+encHeightSize+encHashSize+encPathSize])

	decoded, err := ReadNode(bytes.NewReader(encoded), make([]byte, 1024), noChildNodes(t))
	require.NoError(t, err)
	require.Equal(t, leaf.Height(), decoded.Height())
	require.Equal(t, leaf.Hash(), decoded.Hash())
	require.Equal(t, *leaf.Path(), *decoded.Path())
	require.NotNil(t, decoded.LeafHash())
	require.Equal(t, *leaf.LeafHash(), *decoded.LeafHash())
}

// TestEncodeDecodeLeafNodeWithoutLeafHash covers the `leafHashAbsent` encoding, which is the one
// on-disk mechanism V7 adds over V6. A leaf for an unallocated register has no leaf hash, so the
// flag byte is the only thing recording its absence, and the encoding is 32 bytes shorter.
func TestEncodeDecodeLeafNodeWithoutLeafHash(t *testing.T) {
	path := testutils.PathByUint8(7)

	// An unallocated register (empty value) yields a default leaf, whose leaf hash is nil.
	leaf := NewLeaf(path, nil, 256)
	require.True(t, leaf.IsLeaf())
	require.Nil(t, leaf.LeafHash(), "sanity check: an unallocated register leaf has no leaf hash")

	scratch := make([]byte, 1024)
	encoded := EncodeNode(leaf, 0, 0, scratch)

	// node type (1) + height (2) + hash (32) + path (32) + leaf hash flag (1), and no leaf hash
	require.Len(t, encoded, 68)
	require.Equal(t, leafHashAbsent, encoded[encNodeTypeSize+encHeightSize+encHashSize+encPathSize])

	decoded, err := ReadNode(bytes.NewReader(encoded), make([]byte, 1024), noChildNodes(t))
	require.NoError(t, err)
	require.Equal(t, leaf.Height(), decoded.Height())
	require.Equal(t, leaf.Hash(), decoded.Hash())
	require.Equal(t, *leaf.Path(), *decoded.Path())
	require.Nil(t, decoded.LeafHash(), "absent leaf hash must decode back to nil")
}

// TestReadNodeRejectsInvalidLeafHashFlag verifies that a leaf hash flag other than
// `leafHashAbsent` or `leafHashPresent` is reported as an error rather than silently
// interpreted, so a corrupted checkpoint cannot be read as a valid trie.
func TestReadNodeRejectsInvalidLeafHashFlag(t *testing.T) {
	leaf := NewLeaf(testutils.PathByUint8(7), []byte("register value"), 256)

	encoded := EncodeNode(leaf, 0, 0, make([]byte, 1024))
	flagPos := encNodeTypeSize + encHeightSize + encHashSize + encPathSize

	for _, flag := range []byte{2, 0xff} {
		corrupted := make([]byte, len(encoded))
		copy(corrupted, encoded)
		corrupted[flagPos] = flag

		_, err := ReadNode(bytes.NewReader(corrupted), make([]byte, 1024), noChildNodes(t))
		require.Error(t, err)
		require.ErrorContains(t, err, "invalid leaf hash flag")
	}
}

// TestReadNodeRejectsTruncatedLeafHash verifies that a leaf whose flag promises a leaf hash but
// whose bytes are cut short is reported as an error.
func TestReadNodeRejectsTruncatedLeafHash(t *testing.T) {
	leaf := NewLeaf(testutils.PathByUint8(7), []byte("register value"), 256)

	encoded := EncodeNode(leaf, 0, 0, make([]byte, 1024))
	// drop the last byte of the leaf hash
	truncated := encoded[:len(encoded)-1]

	_, err := ReadNode(bytes.NewReader(truncated), make([]byte, 1024), noChildNodes(t))
	require.Error(t, err)
	require.ErrorContains(t, err, "cannot read leaf hash")
}

// TestEncodeDecodeLeafNodeSmallScratch verifies both leaf encodings are correct when the scratch
// buffer is too small to hold the node, i.e. when the encoder and decoder allocate instead.
func TestEncodeDecodeLeafNodeSmallScratch(t *testing.T) {
	leaves := map[string]*Node{
		"leaf hash present": NewLeaf(testutils.PathByUint8(7), []byte("register value"), 256),
		"leaf hash absent":  NewLeaf(testutils.PathByUint8(7), nil, 256),
	}

	for name, leaf := range leaves {
		t.Run(name, func(t *testing.T) {
			encoded := EncodeNode(leaf, 0, 0, nil)

			decoded, err := ReadNode(bytes.NewReader(encoded), nil, noChildNodes(t))
			require.NoError(t, err)
			require.Equal(t, leaf.Hash(), decoded.Hash())
			require.Equal(t, leaf.LeafHash() == nil, decoded.LeafHash() == nil)
		})
	}
}

// TestEncodeDecodeLeafNodeWithZeroLeafHash guards against a flag-free encoding: an all-zero leaf
// hash is a legitimate value and must not be confused with an absent one.
func TestEncodeDecodeLeafNodeWithZeroLeafHash(t *testing.T) {
	var zeroLeafHash hash.Hash
	path := testutils.PathByUint8(7)
	leaf := NewNode(256, nil, nil, path, &zeroLeafHash, ledger.GetDefaultHashForHeight(0))

	encoded := EncodeNode(leaf, 0, 0, make([]byte, 1024))
	require.Len(t, encoded, 100)

	decoded, err := ReadNode(bytes.NewReader(encoded), make([]byte, 1024), noChildNodes(t))
	require.NoError(t, err)
	require.NotNil(t, decoded.LeafHash(), "a zero leaf hash is present, not absent")
	require.Equal(t, zeroLeafHash, *decoded.LeafHash())
}
