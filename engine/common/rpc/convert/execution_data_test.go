package convert_test

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestConvertBlockExecutionData checks if conversions between BlockExecutionData and it's fields are consistent.
func TestConvertBlockExecutionData(t *testing.T) {
	// Initialize the BlockExecutionData object
	numChunks := 5
	ced := make([]*execution_data.ChunkExecutionData, numChunks)
	bed := &execution_data.BlockExecutionData{
		BlockID:             unittest.IdentifierFixture(),
		ChunkExecutionDatas: ced,
	}

	// Fill the chunk execution datas with trie updates, collections, and events
	minSerializedSize := uint64(10 * execution_data.DefaultMaxBlobSize)
	for i := 0; i < numChunks; i++ {
		// the service chunk sometimes does not have any trie updates
		if i == numChunks-1 {
			tx1 := unittest.TransactionBodyFixture()
			// proposal key and payer are empty addresses for service tx
			tx1.ProposalKey.Address = flow.EmptyAddress
			tx1.Payer = flow.EmptyAddress
			bed.ChunkExecutionDatas[i] = &execution_data.ChunkExecutionData{
				Collection: &flow.Collection{Transactions: []*flow.TransactionBody{&tx1}},
			}
			continue
		}

		// Initialize collection
		tx1 := unittest.TransactionBodyFixture()
		tx2 := unittest.TransactionBodyFixture()
		col := &flow.Collection{Transactions: []*flow.TransactionBody{&tx1, &tx2}}

		// Initialize events
		header := unittest.BlockHeaderFixture()
		events := unittest.BlockEventsFixture(header, 5).Events

		chunk := &execution_data.ChunkExecutionData{
			Collection: col,
			Events:     events,
			TrieUpdate: testutils.TrieUpdateFixture(1, 1, 8),
		}
		size := 1

		// Fill the TrieUpdate with data
	inner:
		for {
			buf := &bytes.Buffer{}
			require.NoError(t, execution_data.DefaultSerializer.Serialize(buf, chunk))

			if buf.Len() >= int(minSerializedSize) {
				break inner
			}

			v := make([]byte, size)
			_, _ = rand.Read(v)

			k, err := chunk.TrieUpdate.Payloads[0].Key()
			require.NoError(t, err)

			chunk.TrieUpdate.Payloads[0] = ledger.NewPayload(k, v)
			size *= 2
		}
		bed.ChunkExecutionDatas[i] = chunk
	}

	t.Run("chunk execution data conversions", func(t *testing.T) {
		chunkMsg, err := convert.ChunkExecutionDataToMessage(bed.ChunkExecutionDatas[0])
		assert.Nil(t, err)

		chunkReConverted, err := convert.MessageToChunkExecutionData(chunkMsg, flow.Testnet.Chain())
		assert.Nil(t, err)
		assert.Equal(t, bed.ChunkExecutionDatas[0], chunkReConverted)
	})

	t.Run("block execution data conversions", func(t *testing.T) {
		blockMsg, err := convert.BlockExecutionDataToMessage(bed)
		assert.Nil(t, err)

		bedReConverted, err := convert.MessageToBlockExecutionData(blockMsg, flow.Testnet.Chain())
		assert.Nil(t, err)
		assert.Equal(t, bed, bedReConverted)
	})
}
