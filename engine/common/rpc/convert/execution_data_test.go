package convert_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestConvertBlockExecutionData1(t *testing.T) {
	t.Parallel()

	chain := flow.Testnet.Chain() // this is used by the AddressFixture
	events := unittest.EventsFixture(5)

	chunks := 5
	chunkData := make([]*execution_data.ChunkExecutionData, 0, chunks)
	for i := 0; i < chunks-1; i++ {
		chunkData = append(chunkData, unittest.ChunkExecutionDataFixture(t, execution_data.DefaultMaxBlobSize/5, unittest.WithChunkEvents(events)))
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
	chunkData = append(chunkData, chunk)

	blockData := unittest.BlockExecutionDataFixture(unittest.WithChunkExecutionDatas(chunkData...))

	t.Run("chunk execution data conversions", func(t *testing.T) {
		chunkMsg, err := convert.ChunkExecutionDataToMessage(chunkData[0])
		require.NoError(t, err)

		chunkReConverted, err := convert.MessageToChunkExecutionData(chunkMsg, flow.Testnet.Chain())
		require.NoError(t, err)

		assert.Equal(t, chunkData[0], chunkReConverted)
	})

	t.Run("block execution data conversions", func(t *testing.T) {
		msg, err := convert.BlockExecutionDataToMessage(blockData)
		require.NoError(t, err)

		converted, err := convert.MessageToBlockExecutionData(msg, chain)
		require.NoError(t, err)

		assert.Equal(t, blockData, converted)
	})
}
