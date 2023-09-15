package execution_data_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/internal"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestSerializeDeserializeChunkExecutionData(t *testing.T) {
	serializer := execution_data.DefaultSerializer

	// Test serializing the current ChunkExecutionData version, then deserializing it back to the
	// same type. Make sure the start and end data structures are the same.
	t.Run("serialize and deserialize ChunkExecutionData", func(t *testing.T) {
		ced := unittest.ChunkExecutionDataFixture(t, 1024, unittest.WithChunkEvents(unittest.EventsFixture(5)))

		buf := new(bytes.Buffer)
		err := serializer.Serialize(buf, ced)
		require.NoError(t, err)

		raw, err := serializer.Deserialize(buf)
		require.NoError(t, err)

		actual, ok := raw.(*execution_data.ChunkExecutionData)
		assert.True(t, ok)
		assert.Equal(t, ced, actual)
	})

	// Test serializing the past ChunkExecutionDataV1 version, then deserializing it to the current
	// ChunkExecutionData version. Make sure the fields in the start and end data structures are
	// the same.
	//
	// This test ensures that the current code is backwards compatible with the previous version of
	// the data structure. It does NOT ensure that the current data structure is backwards compatible
	// with the previous code.
	t.Run("serialize ChunkExecutionDataV1 and deserialize to ChunkExecutionData", func(t *testing.T) {
		cedV2 := unittest.ChunkExecutionDataFixture(t, 1024, unittest.WithChunkEvents(unittest.EventsFixture(5)))
		cedV2.TransactionResults = nil
		cedV1 := &internal.ChunkExecutionDataV1{
			Collection: cedV2.Collection,
			Events:     cedV2.Events,
			TrieUpdate: cedV2.TrieUpdate,
		}

		buf := new(bytes.Buffer)
		err := serializer.Serialize(buf, cedV1)
		require.NoError(t, err)

		raw, err := serializer.Deserialize(buf)
		require.NoError(t, err)

		actual, ok := raw.(*execution_data.ChunkExecutionData)
		assert.True(t, ok)
		assert.Equal(t, cedV2, actual)
	})
}
