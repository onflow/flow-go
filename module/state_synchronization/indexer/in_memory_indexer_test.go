package indexer

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
)

type testFixture struct {
	block     *flow.Block
	exeResult *flow.ExecutionResult
	execData  *execution_data.BlockExecutionData
}

// generateFixture generates a test fixture for the indexer. The returned data has the following
// properties:
//   - The block execution data contains collections for each of the block's guarantees, plus the system chunk
//   - Each collection has 3 transactions
//   - The first path in each trie update is the same, testing that the indexer will use the last value
//   - Every 3rd transaction is failed
//   - There are tx error messages for all failed transactions
func generateFixture(g *fixtures.GeneratorSuite) *testFixture {
	collections := g.Collections().List(4, fixtures.Collection.WithTxCount(3))
	chunkExecutionDatas := make([]*execution_data.ChunkExecutionData, len(collections))
	guarantees := make([]*flow.CollectionGuarantee, len(collections)-1)
	path := g.LedgerPaths().Fixture()
	for i, collection := range collections {
		chunkData := g.ChunkExecutionDatas().Fixture(
			fixtures.ChunkExecutionData.WithCollection(collection),
		)
		// use the same path fo the first ledger payload in each chunk. the indexer should chose the
		// last value in the register entry.
		chunkData.TrieUpdate.Paths[0] = path
		chunkExecutionDatas[i] = chunkData

		if i < len(collections)-1 {
			guarantees[i] = g.Guarantees().Fixture(fixtures.Guarantee.WithCollectionID(collection.ID()))
		}
		for txIndex := range chunkExecutionDatas[i].TransactionResults {
			if txIndex%3 == 0 {
				chunkExecutionDatas[i].TransactionResults[txIndex].Failed = true
			}
		}
	}

	payload := g.Payloads().Fixture(fixtures.Payload.WithGuarantees(guarantees...))
	block := g.Blocks().Fixture(fixtures.Block.WithPayload(payload))

	exeResult := g.ExecutionResults().Fixture(fixtures.ExecutionResult.WithBlock(block))
	execData := g.BlockExecutionDatas().Fixture(
		fixtures.BlockExecutionData.WithBlockID(block.ID()),
		fixtures.BlockExecutionData.WithChunkExecutionDatas(chunkExecutionDatas...),
	)
	return &testFixture{
		block:     block,
		exeResult: exeResult,
		execData:  execData,
	}
}

func assertIndexerData(t *testing.T, indexerData *IndexerData, execData *execution_data.BlockExecutionData) {
	expectedEvents := make([]flow.Event, 0)
	expectedResults := make([]flow.LightTransactionResult, 0)
	expectedCollections := make([]*flow.Collection, 0)
	expectedTransactions := make([]*flow.TransactionBody, 0)
	expectedRegisterEntries := make([]flow.RegisterEntry, 0)

	for i, chunk := range execData.ChunkExecutionDatas {
		expectedEvents = append(expectedEvents, chunk.Events...)
		expectedResults = append(expectedResults, chunk.TransactionResults...)

		if i < len(execData.ChunkExecutionDatas)-1 {
			expectedCollections = append(expectedCollections, chunk.Collection)
			expectedTransactions = append(expectedTransactions, chunk.Collection.Transactions...)
		}

		for j, payload := range chunk.TrieUpdate.Payloads {
			// the first payload of each trie update has the same path. only keep the last one.
			if j == 0 && i < len(execData.ChunkExecutionDatas)-1 {
				continue
			}
			key, value, err := convert.PayloadToRegister(payload)
			require.NoError(t, err)

			expectedRegisterEntries = append(expectedRegisterEntries, flow.RegisterEntry{
				Key:   key,
				Value: value,
			})
		}
	}

	assert.Equal(t, expectedEvents, indexerData.Events)
	assert.Equal(t, expectedResults, indexerData.Results)
	assert.Equal(t, expectedCollections, indexerData.Collections)
	assert.Equal(t, expectedTransactions, indexerData.Transactions)
	assert.ElementsMatch(t, expectedRegisterEntries, indexerData.Registers) // may not be in the same order
}

func TestIndexBlockData(t *testing.T) {
	g := fixtures.NewGeneratorSuite()

	t.Run("happy path", func(t *testing.T) {
		data := generateFixture(g)

		indexer, err := NewInMemoryIndexer(unittest.Logger(), data.block, data.exeResult)
		require.NoError(t, err)

		indexerData, err := indexer.IndexBlockData(data.execData)
		require.NoError(t, err)

		assertIndexerData(t, indexerData, data.execData)
	})

	t.Run("mismatched blockID in constructor", func(t *testing.T) {
		block := g.Blocks().Fixture()
		execResult := g.ExecutionResults().Fixture()

		indexer, err := NewInMemoryIndexer(unittest.Logger(), block, execResult)
		require.Nil(t, indexer)
		require.ErrorContains(t, err, "block ID and execution result block ID must match")
	})

	t.Run("incorrect block ID", func(t *testing.T) {
		data := generateFixture(g)
		block := g.Blocks().Fixture()
		execResult := g.ExecutionResults().Fixture(fixtures.ExecutionResult.WithBlock(block))

		indexer, err := NewInMemoryIndexer(unittest.Logger(), block, execResult)
		require.NoError(t, err)

		indexerData, err := indexer.IndexBlockData(data.execData)
		require.Nil(t, indexerData)
		require.ErrorContains(t, err, "unexpected block execution data: expected block_id")
	})

	t.Run("incorrect chunk count", func(t *testing.T) {
		data := generateFixture(g)
		data.execData.ChunkExecutionDatas = data.execData.ChunkExecutionDatas[:len(data.execData.ChunkExecutionDatas)-1]

		indexer, err := NewInMemoryIndexer(unittest.Logger(), data.block, data.exeResult)
		require.NoError(t, err)

		indexerData, err := indexer.IndexBlockData(data.execData)
		require.Nil(t, indexerData)
		require.ErrorContains(t, err, "block execution data chunk (3) count does not match block guarantee (3) plus system chunk")
	})

	t.Run("mismatched transaction count", func(t *testing.T) {
		data := generateFixture(g)
		data.execData.ChunkExecutionDatas[0].TransactionResults = data.execData.ChunkExecutionDatas[0].TransactionResults[:len(data.execData.ChunkExecutionDatas[0].TransactionResults)-1]

		indexer, err := NewInMemoryIndexer(unittest.Logger(), data.block, data.exeResult)
		require.NoError(t, err)

		indexerData, err := indexer.IndexBlockData(data.execData)
		require.Nil(t, indexerData)
		require.ErrorContains(t, err, "number of transactions (3) does not match number of results (2)")
	})

	t.Run("mismatched ledger path count", func(t *testing.T) {
		data := generateFixture(g)
		data.execData.ChunkExecutionDatas[0].TrieUpdate.Paths = data.execData.ChunkExecutionDatas[0].TrieUpdate.Paths[:len(data.execData.ChunkExecutionDatas[0].TrieUpdate.Paths)-1]

		indexer, err := NewInMemoryIndexer(unittest.Logger(), data.block, data.exeResult)
		require.NoError(t, err)

		indexerData, err := indexer.IndexBlockData(data.execData)
		require.Nil(t, indexerData)
		require.ErrorContains(t, err, "number of ledger paths (1) does not match number of ledger payloads (2)")
	})

	t.Run("invalid register payload", func(t *testing.T) {
		data := generateFixture(g)

		payload := &ledger.Payload{}
		payloadJSON := `{"Key":{"KeyParts":[{"Type":3,"Value":"1c3c5064a9a381ff"},{"Type":2,"Value":"eef4f13ec229f5f7"}]},"Value":"5353ae707c"}`
		err := payload.UnmarshalJSON([]byte(payloadJSON))
		require.NoError(t, err)

		data.execData.ChunkExecutionDatas[0].TrieUpdate.Payloads[1] = payload

		indexer, err := NewInMemoryIndexer(unittest.Logger(), data.block, data.exeResult)
		require.NoError(t, err)

		indexerData, err := indexer.IndexBlockData(data.execData)
		require.Nil(t, indexerData)
		require.ErrorContains(t, err, "failed to convert payload to register entry")
	})
}

func TestValidateTxErrors(t *testing.T) {
	g := fixtures.NewGeneratorSuite()

	data := generateFixture(g)

	indexer, err := NewInMemoryIndexer(unittest.Logger(), data.block, data.exeResult)
	require.NoError(t, err)

	indexerData, err := indexer.IndexBlockData(data.execData)
	require.NoError(t, err)

	txErrMsgs := g.TransactionErrorMessages().ForTransactionResults(indexerData.Results)

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
