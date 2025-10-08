package testutil

import (
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	edprovider "github.com/onflow/flow-go/module/executiondatasync/provider"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
)

type TestFixture struct {
	Block           *flow.Block
	ExecutionResult *flow.ExecutionResult
	ExecutionData   *execution_data.BlockExecutionData
	TxErrorMessages []flow.TransactionResultErrorMessage

	ExpectedEvents                []flow.Event
	ExpectedResults               []flow.LightTransactionResult
	ExpectedCollections           []*flow.Collection
	ExpectedRegisterEntries       flow.RegisterEntries
	ExpectedScheduledTransactions map[flow.Identifier]uint64
}

func newTestFixture(
	t *testing.T,
	block *flow.Block,
	exeResult *flow.ExecutionResult,
	execData *execution_data.BlockExecutionData,
	txErrMsgs []flow.TransactionResultErrorMessage,
	scheduledTransactionIDs []uint64,
) *TestFixture {
	tf := &TestFixture{
		Block:           block,
		ExecutionResult: exeResult,
		ExecutionData:   execData,
		TxErrorMessages: txErrMsgs,

		ExpectedScheduledTransactions: make(map[flow.Identifier]uint64),
	}

	registerEntries := make(map[ledger.Path]flow.RegisterEntry)
	for i, chunkData := range tf.ExecutionData.ChunkExecutionDatas {
		tf.ExpectedEvents = append(tf.ExpectedEvents, chunkData.Events...)
		tf.ExpectedResults = append(tf.ExpectedResults, chunkData.TransactionResults...)
		tf.accumulateRegisterEntries(t, registerEntries, chunkData.TrieUpdate)

		if i < len(tf.ExecutionData.ChunkExecutionDatas)-1 {
			tf.ExpectedCollections = append(tf.ExpectedCollections, chunkData.Collection)
			continue
		}

		// there should be 2 less transactions in the system collection than there are scheduled transactions
		// process callback and system chunk transaction
		require.Equal(t, len(scheduledTransactionIDs), len(chunkData.Collection.Transactions)-2)
		for i, scheduledTransactionID := range scheduledTransactionIDs {
			systemTx := chunkData.Collection.Transactions[i+1]
			tf.ExpectedScheduledTransactions[systemTx.ID()] = scheduledTransactionID
		}
	}
	tf.ExpectedRegisterEntries = slices.Collect(maps.Values(registerEntries))

	return tf
}

func (tf *TestFixture) ExecutionDataEntity() *execution_data.BlockExecutionDataEntity {
	return execution_data.NewBlockExecutionDataEntity(tf.ExecutionResult.ExecutionDataID, tf.ExecutionData)
}

// accumulateRegisterEntries adds all the register entries from a trie update to a map.
// newer entries overwrite older entries.
func (tf *TestFixture) accumulateRegisterEntries(
	t *testing.T,
	registerEntries map[ledger.Path]flow.RegisterEntry,
	update *ledger.TrieUpdate,
) {
	require.Equal(t, len(update.Paths), len(update.Payloads))
	for i, payload := range update.Payloads {
		path := update.Paths[i]

		key, value, err := convert.PayloadToRegister(payload)
		require.NoError(t, err)

		registerEntries[path] = flow.RegisterEntry{
			Key:   key,
			Value: value,
		}
	}
}

// CompleteFixture generates consistent test fixture data for testing BlockExecutionData related
// processing. The returned [TestFixture] includes a [flow.Block], [flow.ExecutionResult] and
// [execution_data.BlockExecutionData], all internally consistent and using consistent randomness.
//
// properties:
//   - The block execution data contains collections for each of the block's guarantees, plus the system chunk
//   - Each collection has 3 transactions
//   - The first path in each trie update is the same, testing that the indexer will use the last value
//   - Every 3rd transaction is failed
//   - There are tx error messages for all failed transactions
//   - There is an execution result for the block, whose ExecutionDataID matches the BlockExecutionData
func CompleteFixture(t *testing.T, g *fixtures.GeneratorSuite, parentHeader *flow.Header) *TestFixture {
	collectionCount := 4
	chunkExecutionDatas := make([]*execution_data.ChunkExecutionData, 0, collectionCount+1)

	// generate the user chunks data
	collections := g.Collections().List(collectionCount, fixtures.Collection.WithTxCount(3))
	guarantees := make([]*flow.CollectionGuarantee, collectionCount)
	path := g.LedgerPaths().Fixture()
	var txErrMsgs []flow.TransactionResultErrorMessage

	txCount := 0
	for i, collection := range collections {
		chunkData := g.ChunkExecutionDatas().Fixture(
			fixtures.ChunkExecutionData.WithCollection(collection),
		)
		// use the same path for the first ledger payload in each chunk. the indexer should chose the
		// last value in the register entry.
		chunkData.TrieUpdate.Paths[0] = path
		chunkExecutionDatas = append(chunkExecutionDatas, chunkData)

		guarantees[i] = g.Guarantees().Fixture(fixtures.Guarantee.WithCollectionID(collection.ID()))
		for txIndex := range chunkExecutionDatas[i].TransactionResults {
			if txIndex%3 == 0 {
				chunkExecutionDatas[i].TransactionResults[txIndex].Failed = true
			}
		}
		txErrMsgs = append(txErrMsgs, g.TransactionErrorMessages().ForTransactionResults(chunkExecutionDatas[i].TransactionResults)...)

		txCount += len(collection.Transactions)
	}
	require.NotEmpty(t, txErrMsgs)

	// generate the system chunk data
	pendingExecutionEvents := make([]flow.Event, 5)
	scheduledTransactionIDs := make([]uint64, 5)
	for i := range 5 {
		id := g.Random().Uint64()
		pendingExecutionEvents[i] = g.PendingExecutionEvents().Fixture(
			fixtures.PendingExecutionEvent.WithTransactionIndex(uint32(txCount)),
			fixtures.PendingExecutionEvent.WithID(id),
		)
		scheduledTransactionIDs[i] = id
	}
	systemCollection, err := blueprints.SystemCollection(g.ChainID().Chain(), pendingExecutionEvents)
	require.NoError(t, err)

	systemResults := g.LightTransactionResults().ForTransactions(systemCollection.Transactions)

	systemChunk := &execution_data.ChunkExecutionData{
		Collection:         systemCollection,
		Events:             pendingExecutionEvents,
		TransactionResults: systemResults,
		TrieUpdate:         g.TrieUpdates().Fixture(),
	}
	chunkExecutionDatas = append(chunkExecutionDatas, systemChunk)

	// generate the block containing guarantees for the user collections
	payload := g.Payloads().Fixture(fixtures.Payload.WithGuarantees(guarantees...))
	block := g.Blocks().Fixture(
		fixtures.Block.WithParentHeader(parentHeader),
		fixtures.Block.WithPayload(payload),
	)

	// generate the block execution data with all data
	execData := g.BlockExecutionDatas().Fixture(
		fixtures.BlockExecutionData.WithBlockID(block.ID()),
		fixtures.BlockExecutionData.WithChunkExecutionDatas(chunkExecutionDatas...),
	)

	executionDataID, _, err := edprovider.NewExecutionDataCIDProvider(execution_data.DefaultSerializer).GenerateExecutionDataRoot(execData)
	require.NoError(t, err)

	exeResult := g.ExecutionResults().Fixture(
		fixtures.ExecutionResult.WithBlock(block),
		fixtures.ExecutionResult.WithExecutionDataID(executionDataID),
	)

	return newTestFixture(t, block, exeResult, execData, txErrMsgs, scheduledTransactionIDs)
}
