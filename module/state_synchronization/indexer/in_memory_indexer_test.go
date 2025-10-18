package indexer

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/testutil"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
)

func TestIndexBlockData(t *testing.T) {
	g := fixtures.NewGeneratorSuite()

	t.Run("happy path", func(t *testing.T) {
		data := testutil.CompleteFixture(t, g, g.Blocks().Fixture())

		indexer, err := NewInMemoryIndexer(unittest.Logger(), data.Block, data.ExecutionResult)
		require.NoError(t, err)

		indexerData, err := indexer.IndexBlockData(data.ExecutionData)
		require.NoError(t, err)

		assert.Equal(t, data.ExpectedEvents, indexerData.Events)
		assert.Equal(t, data.ExpectedResults, indexerData.Results)
		assert.Equal(t, data.ExpectedCollections, indexerData.Collections)
		assert.Equal(t, data.ExpectedScheduledTransactions, indexerData.ScheduledTransactions)
		assert.ElementsMatch(t, data.ExpectedRegisterEntries, indexerData.Registers) // may not be in the same order
	})

	t.Run("mismatched blockID in constructor", func(t *testing.T) {
		block := g.Blocks().Fixture()
		execResult := g.ExecutionResults().Fixture()

		indexer, err := NewInMemoryIndexer(unittest.Logger(), block, execResult)
		require.Nil(t, indexer)
		require.ErrorContains(t, err, "block ID and execution result block ID must match")
	})

	t.Run("incorrect block ID", func(t *testing.T) {
		data := testutil.CompleteFixture(t, g, g.Blocks().Fixture())

		block := g.Blocks().Fixture()
		execResult := g.ExecutionResults().Fixture(fixtures.ExecutionResult.WithBlock(block))

		indexer, err := NewInMemoryIndexer(unittest.Logger(), block, execResult)
		require.NoError(t, err)

		indexerData, err := indexer.IndexBlockData(data.ExecutionData)
		require.Nil(t, indexerData)
		require.ErrorContains(t, err, "unexpected block execution data: expected block_id")
	})

	t.Run("incorrect chunk count", func(t *testing.T) {
		data := testutil.CompleteFixture(t, g, g.Blocks().Fixture())

		data.ExecutionData.ChunkExecutionDatas = data.ExecutionData.ChunkExecutionDatas[:len(data.ExecutionData.ChunkExecutionDatas)-1]

		indexer, err := NewInMemoryIndexer(unittest.Logger(), data.Block, data.ExecutionResult)
		require.NoError(t, err)

		indexerData, err := indexer.IndexBlockData(data.ExecutionData)
		require.Nil(t, indexerData)
		require.ErrorContains(t, err, "block execution data chunk (4) count does not match block guarantee (4) plus system chunk")
	})

	t.Run("mismatched transaction count", func(t *testing.T) {
		data := testutil.CompleteFixture(t, g, g.Blocks().Fixture())
		data.ExecutionData.ChunkExecutionDatas[0].TransactionResults = data.ExecutionData.ChunkExecutionDatas[0].TransactionResults[:len(data.ExecutionData.ChunkExecutionDatas[0].TransactionResults)-1]

		indexer, err := NewInMemoryIndexer(unittest.Logger(), data.Block, data.ExecutionResult)
		require.NoError(t, err)

		indexerData, err := indexer.IndexBlockData(data.ExecutionData)
		require.Nil(t, indexerData)
		require.ErrorContains(t, err, "number of transactions (3) does not match number of results (2)")
	})

	t.Run("mismatched ledger path count", func(t *testing.T) {
		data := testutil.CompleteFixture(t, g, g.Blocks().Fixture())
		data.ExecutionData.ChunkExecutionDatas[0].TrieUpdate.Paths = data.ExecutionData.ChunkExecutionDatas[0].TrieUpdate.Paths[:len(data.ExecutionData.ChunkExecutionDatas[0].TrieUpdate.Paths)-1]

		indexer, err := NewInMemoryIndexer(unittest.Logger(), data.Block, data.ExecutionResult)
		require.NoError(t, err)

		indexerData, err := indexer.IndexBlockData(data.ExecutionData)
		require.Nil(t, indexerData)
		require.ErrorContains(t, err, "number of ledger paths (1) does not match number of ledger payloads (2)")
	})

	t.Run("invalid register payload", func(t *testing.T) {
		data := testutil.CompleteFixture(t, g, g.Blocks().Fixture())

		payload := &ledger.Payload{}
		payloadJSON := `{"Key":{"KeyParts":[{"Type":3,"Value":"1c3c5064a9a381ff"},{"Type":2,"Value":"eef4f13ec229f5f7"}]},"Value":"5353ae707c"}`
		err := payload.UnmarshalJSON([]byte(payloadJSON))
		require.NoError(t, err)

		data.ExecutionData.ChunkExecutionDatas[0].TrieUpdate.Payloads[1] = payload

		indexer, err := NewInMemoryIndexer(unittest.Logger(), data.Block, data.ExecutionResult)
		require.NoError(t, err)

		indexerData, err := indexer.IndexBlockData(data.ExecutionData)
		require.Nil(t, indexerData)
		require.ErrorContains(t, err, "failed to convert payload to register entry")
	})
}

func TestValidateTxErrors(t *testing.T) {
	g := fixtures.NewGeneratorSuite()

	data := testutil.CompleteFixture(t, g, g.Blocks().Fixture())
	txErrMsgs := data.TxErrorMessages

	indexer, err := NewInMemoryIndexer(unittest.Logger(), data.Block, data.ExecutionResult)
	require.NoError(t, err)

	indexerData, err := indexer.IndexBlockData(data.ExecutionData)
	require.NoError(t, err)

	t.Run("happy path", func(t *testing.T) {
		err = ValidateTxErrors(indexerData.Results, txErrMsgs)
		require.NoError(t, err)
	})

	t.Run("missing tx error messages", func(t *testing.T) {
		errMsg := txErrMsgs[len(txErrMsgs)-1]
		txErrMsgs := txErrMsgs[:len(txErrMsgs)-1]

		err = ValidateTxErrors(indexerData.Results, txErrMsgs)
		assert.ErrorContains(t, err, fmt.Sprintf("transaction %s failed but no error message was provided", errMsg.TransactionID))
	})

	t.Run("mismatched tx error message count", func(t *testing.T) {
		txErrMsgs := append(txErrMsgs, flow.TransactionResultErrorMessage{
			TransactionID: g.Identifiers().Fixture(),
			Index:         g.Random().Uint32(),
			ErrorMessage:  "test error",
			ExecutorID:    g.Identifiers().Fixture(),
		})

		err = ValidateTxErrors(indexerData.Results, txErrMsgs)
		assert.ErrorContains(t, err, "number of failed transactions (4) does not match number of transaction error messages (5)")
	})

	t.Run("mismatched tx error message transaction ID", func(t *testing.T) {
		txErrMsgs := make([]flow.TransactionResultErrorMessage, len(txErrMsgs))
		copy(txErrMsgs, txErrMsgs)
		txErrMsgs[0].TransactionID = g.Identifiers().Fixture()

		err = ValidateTxErrors(indexerData.Results, txErrMsgs)
		assert.ErrorContains(t, err, "failed but no error message was provided")
	})
}
