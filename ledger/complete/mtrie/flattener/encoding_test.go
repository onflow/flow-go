package flattener_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/encoding"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
)

func TestStorableNode(t *testing.T) {
	path := utils.PathByUint8(3)
	payload := utils.LightPayload8('A', 'a')
	hashValue := hash.Hash([32]byte{4, 4, 4})

	n := node.NewNode(2137, nil, nil, path, payload, hashValue, 7, 5000)

	storableNode := &flattener.StorableNode{
		LIndex:     1,
		RIndex:     2,
		Height:     2137,
		Path:       path[:],
		EncPayload: encoding.EncodePayload(payload),
		HashValue:  hashValue[:],
		MaxDepth:   7,
		RegCount:   5000,
	}

	// Version 0
	expected := []byte{
		0, 0, // encoding version
		8, 89, // height
		0, 0, 0, 0, 0, 0, 0, 1, // LIndex
		0, 0, 0, 0, 0, 0, 0, 2, // RIndex
		0, 7, // max depth
		0, 0, 0, 0, 0, 0, 19, 136, // reg count
		0, 32, // path data len
		3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // path data
		0, 0, 0, 25, // payload data len
		0, 0, 6, 0, 0, 0, 9, 0, 1, 0, 0, 0, 3, 0, 0, 65, 0, 0, 0, 0, 0, 0, 0, 1, 97, // payload data
		0, 32, // hashValue length
		4, 4, 4, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, // hashValue
	}

	t.Run("encode", func(t *testing.T) {
		data := flattener.EncodeNode(n, 1, 2)
		assert.Equal(t, expected, data)
	})

	t.Run("decode", func(t *testing.T) {
		reader := bytes.NewReader(expected)
		newStorableNode, err := flattener.ReadStorableNode(reader)
		require.NoError(t, err)
		assert.Equal(t, storableNode, newStorableNode)
	})
}

func TestStorableTrie(t *testing.T) {
	hashValue := hash.Hash([32]byte{2, 2, 2})

	rootNode := node.NewNode(256, nil, nil, ledger.DummyPath, nil, hashValue, 7, 5000)

	storableTrie := &flattener.StorableTrie{
		RootIndex: 21,
		RootHash:  hashValue[:],
	}

	// Version 0
	expected := []byte{
		0, 0, // encoding version
		0, 0, 0, 0, 0, 0, 0, 21, // RootIndex
		0, 32, // RootHash length
		2, 2, 2, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, // RootHash data
	}

	t.Run("encode", func(t *testing.T) {
		data := flattener.EncodeTrie(rootNode, 21)

		assert.Equal(t, expected, data)
	})

	t.Run("decode", func(t *testing.T) {

		reader := bytes.NewReader(expected)

		newStorableNode, err := flattener.ReadStorableTrie(reader)
		require.NoError(t, err)

		assert.Equal(t, storableTrie, newStorableNode)
	})

}
