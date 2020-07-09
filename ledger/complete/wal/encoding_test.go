package wal

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/ledger/complete/mtrie/flattener"
)

func TestUpdate(t *testing.T) {

	stateComm := []byte{2, 1, 3, 7}

	keys := [][]byte{
		{1, 2},
		{3, 4},
		{5, 6},
	}

	vals := [][]byte{
		{1},
		{1, 2, 3, 4, 5, 6},
		{1, 2},
	}

	expected := []byte{
		1,    //update flag
		0, 4, //state commit length
		2, 1, 3, 7, //state commit data
		0, 0, 0, 3, //records number
		0, 2, // key size
		1, 2, // first key
		0, 0, 0, 1, //first val len
		1,                                  //first val
		3, 4, 0, 0, 0, 6, 1, 2, 3, 4, 5, 6, //second key + val len + val
		5, 6, 0, 0, 0, 2, 1, 2, //third key + val len + val
	}

	t.Run("size", func(t *testing.T) {
		size := updateSize(stateComm, keys, vals)

		//header + record count + key size + (3 * key expected) + ((4 + 1) + (4 + 6) + (4 + 2)) values with sizes
		assert.Equal(t, 7+4+2+(3*2)+((4+1)+(4+6)+(4+2)), size)

	})

	t.Run("encode", func(t *testing.T) {
		data := EncodeUpdate(stateComm, keys, vals)

		assert.Equal(t, expected, data)
	})

	t.Run("decode", func(t *testing.T) {

		operation, stateCommitment, decodedKeys, decodedValues, err := Decode(expected)
		require.NoError(t, err)

		assert.Equal(t, WALUpdate, operation)
		assert.Equal(t, stateComm, stateCommitment)
		assert.Equal(t, keys, decodedKeys)
		assert.Equal(t, vals, decodedValues)

	})

}
func TestStorableNode(t *testing.T) {

	storableNode := &flattener.StorableNode{
		LIndex:    0,
		RIndex:    1,
		Height:    2137,
		Key:       []byte{2, 2, 2},
		Value:     []byte{3, 3, 3},
		HashValue: []byte{4, 4, 4},
	}

	expected := []byte{
		8, 89, // height
		0, 0, 0, 0, 0, 0, 0, 0, // LIndex
		0, 0, 0, 0, 0, 0, 0, 1, // RIndex
		0, 3, 2, 2, 2, // key length + data
		0, 0, 0, 3, 3, 3, 3, // val length + data
		0, 3, 4, 4, 4, // hashValue length + data
	}

	t.Run("encode", func(t *testing.T) {
		data := EncodeStorableNode(storableNode)

		assert.Equal(t, expected, data)
	})

	t.Run("decode", func(t *testing.T) {

		reader := bytes.NewReader(expected)

		newStorableNode, err := ReadStorableNode(reader)
		require.NoError(t, err)

		assert.Equal(t, storableNode, newStorableNode)
	})

}

func TestStorableTrie(t *testing.T) {

	storableTrie := &flattener.StorableTrie{
		RootIndex: 21,
		RootHash:  []byte{2, 2, 2},
	}

	expected := []byte{
		0, 0, 0, 0, 0, 0, 0, 21, // RootIndex
		0, 3, 2, 2, 2, // RootHash length + data
	}

	t.Run("encode", func(t *testing.T) {
		data := EncodeStorableTrie(storableTrie)

		assert.Equal(t, expected, data)
	})

	t.Run("decode", func(t *testing.T) {

		reader := bytes.NewReader(expected)

		newStorableNode, err := ReadStorableTrie(reader)
		require.NoError(t, err)

		assert.Equal(t, storableTrie, newStorableNode)
	})

}

func TestDelete(t *testing.T) {

	stateComm := []byte{2, 1, 3, 7}

	expected := []byte{
		2,    //delete flag
		0, 4, //state commit length
		2, 1, 3, 7, //state commit data
	}

	t.Run("size", func(t *testing.T) {
		size := deleteSize(stateComm)

		// 1 op + 2 state comm size + data
		assert.Equal(t, 7, size)

	})

	t.Run("encode", func(t *testing.T) {
		data := EncodeDelete(stateComm)

		assert.Equal(t, expected, data)
	})

	t.Run("decode", func(t *testing.T) {

		operation, stateCommitment, _, _, err := Decode(expected)
		require.NoError(t, err)

		assert.Equal(t, WALDelete, operation)
		assert.Equal(t, stateComm, stateCommitment)

	})

}
