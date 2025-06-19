package optimistic_sync

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	txerrmsgsmock "github.com/onflow/flow-go/engine/access/ingestion/tx_error_messages/mock"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/metrics"
	reqestermock "github.com/onflow/flow-go/module/state_synchronization/requester/mock"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/operation/badgerimpl"
	pebbleStorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

type PipelineFunctionalSuite struct {
	suite.Suite
	logger                        zerolog.Logger
	execDataRequester             *reqestermock.ExecutionDataRequester
	txResultErrMsgsRequester      *txerrmsgsmock.TransactionResultErrorMessageRequester
	txResultErrMsgsRequestTimeout time.Duration
	tmpDir                        string
	bdb                           *badger.DB
	pdb                           *pebble.DB
	persistentRegisters           *pebbleStorage.Registers
	persistentEvents              *store.Events
	persistentCollections         *store.Collections
	persistentTransactions        *store.Transactions
	persistentResults             *store.LightTransactionResults
	persistentTxResultErrMsg      *store.TransactionResultErrorMessages
	core                          *CoreImpl
	block                         *flow.Block
	executionResult               *flow.ExecutionResult
	metrics                       module.CacheMetrics
}

func TestPipelineFunctionalSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(PipelineFunctionalSuite))
}

func (p *PipelineFunctionalSuite) SetupTest() {
	t := p.T()

	p.tmpDir = unittest.TempDir(t)
	p.logger = zerolog.Nop()
	p.metrics = metrics.NewNoopCollector()
	p.bdb = unittest.BadgerDB(t, p.tmpDir)
	db := badgerimpl.ToDB(p.bdb)

	rootBlock := unittest.BlockHeaderFixture()

	// Create real storages
	var err error
	p.pdb = pebbleStorage.NewBootstrappedRegistersWithPathForTest(t, p.tmpDir, rootBlock.Height, rootBlock.Height)
	p.persistentRegisters, err = pebbleStorage.NewRegisters(p.pdb, pebbleStorage.PruningDisabled)
	p.Require().NoError(err)

	p.persistentEvents = store.NewEvents(p.metrics, db)
	p.persistentTransactions = store.NewTransactions(p.metrics, db)
	p.persistentCollections = store.NewCollections(db, p.persistentTransactions)
	p.persistentResults = store.NewLightTransactionResults(p.metrics, db, bstorage.DefaultCacheSize)
	p.persistentTxResultErrMsg = store.NewTransactionResultErrorMessages(p.metrics, db, bstorage.DefaultCacheSize)

	p.block = unittest.BlockWithParentFixture(rootBlock)
	p.executionResult = unittest.ExecutionResultFixture(unittest.WithBlock(p.block))

	p.execDataRequester = reqestermock.NewExecutionDataRequester(t)
	p.txResultErrMsgsRequester = txerrmsgsmock.NewTransactionResultErrorMessageRequester(t)
	p.txResultErrMsgsRequestTimeout = DefaultTxResultErrMsgsRequestTimeout

	p.core = NewCoreImpl(
		p.logger,
		p.executionResult,
		p.block.Header,
		p.execDataRequester,
		p.txResultErrMsgsRequester,
		p.txResultErrMsgsRequestTimeout,
		p.persistentRegisters,
		p.persistentEvents,
		p.persistentCollections,
		p.persistentTransactions,
		p.persistentResults,
		p.persistentTxResultErrMsg,
		db,
	)
}

func (p *PipelineFunctionalSuite) TearDownTest() {
	p.Require().NoError(p.pdb.Close())
	p.Require().NoError(p.bdb.Close())
	p.Require().NoError(os.RemoveAll(p.tmpDir))
}

// TestPipelineHappyCase verifies the complete happy path flow:
// 1. Pipeline processes execution data through all states
// 2. All data types are correctly persisted to storage
// 3. State transitions occur in the expected order
func (p *PipelineFunctionalSuite) TestPipelineHappyCase() {
	expectedChunkExecutionData := unittest.ChunkExecutionDataFixture(
		p.T(),
		0,
		unittest.WithChunkEvents(unittest.EventsFixture(5)),
		unittest.WithTrieUpdate(createTestTrieUpdate(p.T())),
	)
	systemChunkCollection := unittest.CollectionFixture(1)
	systemChunkData := &execution_data.ChunkExecutionData{
		Collection: &systemChunkCollection,
	}

	expectedExecutionData := unittest.BlockExecutionDataFixture(
		unittest.WithBlockExecutionDataBlockID(p.block.ID()),
		unittest.WithChunkExecutionDatas(expectedChunkExecutionData, systemChunkData),
	)
	p.execDataRequester.On("RequestExecutionData", mock.Anything).Return(expectedExecutionData, nil).Once()

	expectedTxResultErrMsgs := unittest.TransactionResultErrorMessagesFixture(5)
	p.txResultErrMsgsRequester.On("Request", mock.Anything).Return(expectedTxResultErrMsgs, nil).Once()

	// Create channels for state updates
	updateChan := make(chan State, 10)

	// Create publisher function
	publisher := func(state State) {
		updateChan <- state
	}

	// Create a pipeline
	pipeline := NewPipeline(p.logger, false, p.executionResult, p.core, publisher)

	// Start the pipeline in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error)
	go func() {
		errChan <- pipeline.Run(ctx, p.core)
	}()

	pipeline.OnParentStateUpdated(StateComplete)

mainLoop:
	for {
		select {
		case err := <-errChan:
			p.Require().NoError(err)
			return // Exit after pipeline completes
		case newState := <-updateChan:
			p.Require().NotEqual(StateAbandoned, newState)

			switch newState {
			case StateWaitingPersist:
				pipeline.SetSealed()
			case StateComplete:
				break mainLoop
			default:
				continue
			}
		}
	}

	// Verify all data was actually persisted to storage
	p.verifyDataPersistence(expectedChunkExecutionData, expectedTxResultErrMsgs)
}

// 2. Various error conditions are handled
// 3. Cancelation during various stages
// 4. Graceful shutdowns

// verifyDataPersistence checks that all expected data was actually persisted to storage
func (p *PipelineFunctionalSuite) verifyDataPersistence(
	expectedChunkExecutionData *execution_data.ChunkExecutionData,
	expectedTxResultErrMsgs []flow.TransactionResultErrorMessage,
) {
	p.verifyEventsPersisted(expectedChunkExecutionData.Events)

	p.verifyCollectionPersisted(expectedChunkExecutionData.Collection)

	p.verifyTransactionResultsPersisted(expectedChunkExecutionData.TransactionResults)

	p.verifyRegistersPersisted(expectedChunkExecutionData.TrieUpdate)

	p.verifyTxResultErrorMessagesPersisted(expectedTxResultErrMsgs)
}

// verifyEventsPersisted checks that events were stored correctly
func (p *PipelineFunctionalSuite) verifyEventsPersisted(expectedEvents flow.EventsList) {
	storedEvents, err := p.persistentEvents.ByBlockID(p.block.ID())
	p.Require().NoError(err)

	p.Assert().Equal(expectedEvents, flow.EventsList(storedEvents))
}

// verifyCollectionPersisted checks that the collection was stored correctly
func (p *PipelineFunctionalSuite) verifyCollectionPersisted(expectedCollection *flow.Collection) {
	collectionID := expectedCollection.ID()
	expectedLightCollection := expectedCollection.Light()

	storedLightCollection, err := p.persistentCollections.LightByID(collectionID)
	p.Require().NoError(err)

	p.Assert().Equal(&expectedLightCollection, storedLightCollection)
	p.Assert().ElementsMatch(expectedCollection.Light().Transactions, storedLightCollection.Transactions)
}

// verifyTransactionResultsPersisted checks that transaction results were stored correctly
func (p *PipelineFunctionalSuite) verifyTransactionResultsPersisted(expectedResults []flow.LightTransactionResult) {
	storedResults, err := p.persistentResults.ByBlockID(p.block.ID())
	p.Require().NoError(err)

	p.Assert().ElementsMatch(expectedResults, storedResults)
}

// verifyRegistersPersisted checks that registers were stored correctly
func (p *PipelineFunctionalSuite) verifyRegistersPersisted(expectedTrieUpdate *ledger.TrieUpdate) {
	for _, payload := range expectedTrieUpdate.Payloads {
		key, err := payload.Key()
		p.Require().NoError(err)

		registerID, err := convert.LedgerKeyToRegisterID(key)
		p.Require().NoError(err)

		storedValue, err := p.persistentRegisters.Get(registerID, p.block.Header.Height)
		p.Require().NoError(err)

		expectedValue := payload.Value()
		p.Assert().Equal(expectedValue, ledger.Value(storedValue))
	}
}

// verifyTxResultErrorMessagesPersisted checks that transaction result error messages were stored correctly
func (p *PipelineFunctionalSuite) verifyTxResultErrorMessagesPersisted(expectedTxResultErrMsgs []flow.TransactionResultErrorMessage) {
	storedErrMsgs, err := p.persistentTxResultErrMsg.ByBlockID(p.block.ID())
	p.Require().NoError(err, "Should be able to retrieve tx result error messages by block ID")

	p.Assert().ElementsMatch(expectedTxResultErrMsgs, storedErrMsgs)
}

func createTestTrieUpdate(t *testing.T) *ledger.TrieUpdate {
	return createTestTrieWithPayloads(
		[]*ledger.Payload{
			createTestPayload(t),
			createTestPayload(t),
			createTestPayload(t),
			createTestPayload(t),
		})
}

func createTestTrieWithPayloads(payloads []*ledger.Payload) *ledger.TrieUpdate {
	keys := make([]ledger.Key, 0)
	values := make([]ledger.Value, 0)
	for _, payload := range payloads {
		key, _ := payload.Key()
		keys = append(keys, key)
		values = append(values, payload.Value())
	}

	update, _ := ledger.NewUpdate(ledger.DummyState, keys, values)
	trie, _ := pathfinder.UpdateToTrieUpdate(update, complete.DefaultPathFinderVersion)
	return trie
}

func createTestPayload(t *testing.T) *ledger.Payload {
	owner := unittest.RandomAddressFixture()
	key := make([]byte, 8)
	_, err := rand.Read(key)
	require.NoError(t, err)
	val := make([]byte, 8)
	_, err = rand.Read(val)
	require.NoError(t, err)
	return createTestLedgerPayload(owner.String(), fmt.Sprintf("%x", key), val)
}

func createTestLedgerPayload(owner string, key string, value []byte) *ledger.Payload {
	k := ledger.Key{
		KeyParts: []ledger.KeyPart{
			{
				Type:  ledger.KeyPartOwner,
				Value: []byte(owner),
			},
			{
				Type:  ledger.KeyPartKey,
				Value: []byte(key),
			},
		},
	}

	return ledger.NewPayload(k, value)
}
