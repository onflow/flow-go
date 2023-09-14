package convert_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestConvertBlockExecutionData(t *testing.T) {
	t.Parallel()

	chain := flow.Testnet.Chain() // this is used by the AddressFixture
	events := unittest.EventsFixture(5)

	chunks := 5
	chunkData := make([]*execution_data.ChunkExecutionData, 0, chunks)
	for i := 0; i < chunks-1; i++ {
		ced := unittest.ChunkExecutionDataFixture(t,
			0, // updates set explicitly to target 160-320KB per chunk
			unittest.WithChunkEvents(events),
			unittest.WithTrieUpdate(testutils.TrieUpdateFixture(5, 32*1024, 64*1024)),
		)

		// TODO: remove this line after adding TransactionResult to the ChunkExecutionData entity
		ced.TransactionResults = nil

		chunkData = append(chunkData, ced)
	}
	makeServiceTx := func(ced *execution_data.ChunkExecutionData) {
		// proposal key and payer are empty addresses for service tx
		collection := unittest.CollectionFixture(1)
		collection.Transactions[0].ProposalKey.Address = flow.EmptyAddress
		collection.Transactions[0].Payer = flow.EmptyAddress
		ced.Collection = &collection

		// the service chunk sometimes does not have any trie updates
		ced.TrieUpdate = nil
	}
	chunk := unittest.ChunkExecutionDataFixture(t, execution_data.DefaultMaxBlobSize/5, unittest.WithChunkEvents(events), makeServiceTx)
	// TODO: remove this line after adding TransactionResult to the ChunkExecutionData entity
	chunk.TransactionResults = nil
	chunkData = append(chunkData, chunk)

	blockData := unittest.BlockExecutionDataFixture(unittest.WithChunkExecutionDatas(chunkData...))

	t.Run("chunk execution data conversions", func(t *testing.T) {
		chunkMsg, err := convert.ChunkExecutionDataToMessage(chunkData[0])
		require.NoError(t, err)

		chunkReConverted, err := convert.MessageToChunkExecutionData(chunkMsg, flow.Testnet.Chain())
		require.NoError(t, err)

		assert.Equal(t, chunkData[0], chunkReConverted)
		assert.True(t, chunkData[0].TrieUpdate.Equals(chunkReConverted.TrieUpdate))
	})

	t.Run("block execution data conversions", func(t *testing.T) {
		msg, err := convert.BlockExecutionDataToMessage(blockData)
		require.NoError(t, err)

		converted, err := convert.MessageToBlockExecutionData(msg, chain)
		require.NoError(t, err)

		assert.Equal(t, blockData, converted)
		for i, chunk := range blockData.ChunkExecutionDatas {
			if chunk.TrieUpdate == nil {
				assert.Nil(t, converted.ChunkExecutionDatas[i].TrieUpdate)
			} else {
				assert.True(t, chunk.TrieUpdate.Equals(converted.ChunkExecutionDatas[i].TrieUpdate))
			}
		}
	})
}
