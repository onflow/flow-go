package wal_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	realWAL "github.com/onflow/flow-go/ledger/complete/wal"
)

func TestUpdate(t *testing.T) {

	var rootHash ledger.RootHash
	copy(rootHash[:], []byte{2, 1, 3, 7})
	p1 := ledger.PathByUint16(uint16(1))
	p2 := ledger.PathByUint16(uint16(772))
	paths := []ledger.Path{p1, p2}
	v1 := ledger.LightPayload8(1, 2)
	v2 := ledger.LightPayload(2, 3)
	payloads := []*ledger.Payload{v1, v2}
	update := &ledger.TrieUpdate{RootHash: rootHash, Paths: paths, Payloads: payloads}

	expected := []byte{
		1, //update flag,
		0, 0, 11, 0, 32, 2, 1, 3, 7, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 0, 32,
		0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		3, 4, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		22, 0, 0, 0, 9, 0, 1, 0, 0, 0, 3, 0, 0, 1, 0, 0,
		0, 0, 0, 0, 0, 1, 2, 0, 0, 0, 24, 0, 0, 0, 10, 0, 1, 0, 0,
		0, 4, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0, 0, 2, 0, 3, // encoded update
	}

	t.Run("encode", func(t *testing.T) {
		data := realWAL.EncodeUpdate(update)
		assert.Equal(t, expected, data)
	})

	t.Run("decode", func(t *testing.T) {
		data := realWAL.EncodeUpdate(update)
		operation, stateCommitment, up, err := realWAL.Decode(data)
		require.NoError(t, err)
		assert.Equal(t, realWAL.WALUpdate, operation)
		assert.Equal(t, stateCommitment, ledger.RootHash(hash.DummyHash))
		assert.Equal(t, update, up)
	})
}

func TestDelete(t *testing.T) {

	var rootHash ledger.RootHash
	copy(rootHash[:], []byte{2, 1, 3, 7})

	expected := []byte{
		2,     // delete flag
		0, 32, // root hash length
		2, 1, 3, 7, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // root hash data
	}

	t.Run("encode", func(t *testing.T) {
		data := realWAL.EncodeDelete(rootHash)
		assert.Equal(t, expected, data)
	})

	t.Run("decode", func(t *testing.T) {
		operation, rootH, _, err := realWAL.Decode(expected)
		require.NoError(t, err)
		assert.Equal(t, realWAL.WALDelete, operation)
		assert.Equal(t, rootHash, rootH)
	})

}
