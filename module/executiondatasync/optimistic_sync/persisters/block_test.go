package persisters

import (
	"errors"
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync/persisters/stores"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
)

type PersisterSuite struct {
	suite.Suite

	headers          *storagemock.Headers
	executionResults *storagemock.ExecutionResults

	executionResult *flow.ExecutionResult
	header          *flow.Header
	indexerData     *indexer.IndexerData
	txErrMsgs       []flow.TransactionResultErrorMessage
}

func TestPersisterSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(PersisterSuite))
}

func (p *PersisterSuite) SetupTest() {
	g := fixtures.NewGeneratorSuite()

	block := g.Blocks().Fixture()
	p.header = block.ToHeader()
	p.executionResult = g.ExecutionResults().Fixture(fixtures.ExecutionResult.WithBlock(block))

	p.indexerData = &indexer.IndexerData{
		Events:       g.Events().List(5),
		Collections:  g.Collections().List(2),
		Transactions: g.Transactions().List(2),
		Results:      g.LightTransactionResults().List(4),
		Registers:    g.RegisterEntries().List(3),
	}

	for txIndex := range p.indexerData.Results {
		if txIndex%2 == 0 {
			p.indexerData.Results[txIndex].Failed = true
		}
	}
	p.txErrMsgs = g.TransactionErrorMessages().ForTransactionResults(p.indexerData.Results)
}

func (p *PersisterSuite) TestPersister_HappyPath() {
	p.testWithDatabase()
}

func (p *PersisterSuite) TestPersister_EmptyData() {
	p.indexerData = &indexer.IndexerData{}
	p.txErrMsgs = nil
	p.testWithDatabase()
}

func (p *PersisterSuite) testWithDatabase() {
	logger := unittest.Logger()
	metrics := metrics.NewNoopCollector()
	lockManager := storage.NewTestingLockManager()

	p.headers = storagemock.NewHeaders(p.T())
	p.headers.On("ByHeight", p.header.Height).Return(p.header, nil)

	p.executionResults = storagemock.NewExecutionResults(p.T())
	p.executionResults.On("ByBlockID", p.executionResult.BlockID).Return(p.executionResult, nil)

	unittest.RunWithPebbleDB(p.T(), func(pdb *pebble.DB) {
		db := pebbleimpl.ToDB(pdb)

		events := store.NewEvents(metrics, db)
		results := store.NewLightTransactionResults(metrics, db, store.DefaultCacheSize)
		transactions := store.NewTransactions(metrics, db)
		collections := store.NewCollections(db, transactions)
		txResultErrMsg := store.NewTransactionResultErrorMessages(metrics, db, store.DefaultCacheSize)

		progress, err := store.NewConsumerProgress(db, "test_consumer").Initialize(p.header.Height)
		p.Require().NoError(err)

		latestPersistedSealedResult, err := store.NewLatestPersistedSealedResult(progress, p.headers, p.executionResults)
		p.Require().NoError(err)

		persister := NewBlockPersister(
			logger,
			db,
			lockManager,
			p.executionResult,
			[]stores.PersisterStore{
				stores.NewEventsStore(p.indexerData.Events, events, p.executionResult.BlockID),
				stores.NewResultsStore(p.indexerData.Results, results, p.executionResult.BlockID),
				stores.NewCollectionsStore(p.indexerData.Collections, collections, lockManager),
				stores.NewTxResultErrMsgStore(p.txErrMsgs, txResultErrMsg, p.executionResult.BlockID),
				stores.NewLatestSealedResultStore(latestPersistedSealedResult, p.executionResult.ID(), p.header.Height),
			},
		)

		err = persister.Persist()
		p.Require().NoError(err)

		// Assert all of the expected data exists in the database
		blockEvents, err := events.ByBlockID(p.executionResult.BlockID)
		p.Require().NoError(err)
		p.Require().Equal(p.indexerData.Events, blockEvents)

		blockTxResults, err := results.ByBlockID(p.executionResult.BlockID)
		p.Require().NoError(err)
		p.Require().Equal(p.indexerData.Results, blockTxResults)

		for _, expectedCollection := range p.indexerData.Collections {
			expectedLightCollection := expectedCollection.Light()
			expectedID := expectedCollection.ID()

			actualCollection, err := collections.ByID(expectedID)
			p.Require().NoError(err)
			p.Require().Equal(expectedCollection, actualCollection)

			actualLightCollection, err := collections.LightByID(expectedID)
			p.Require().NoError(err)
			p.Require().Equal(expectedLightCollection, actualLightCollection)

			for i, txID := range expectedLightCollection.Transactions {
				tx, err := transactions.ByID(txID)
				p.Require().NoError(err)
				p.Require().Equal(expectedCollection.Transactions[i], tx)
			}
		}

		blockTxResultErrMsgs, err := txResultErrMsg.ByBlockID(p.executionResult.BlockID)
		p.Require().NoError(err)
		require.Equal(p.T(), p.txErrMsgs, blockTxResultErrMsgs)

		resultID, height := latestPersistedSealedResult.Latest()
		p.Require().Equal(p.executionResult.ID(), resultID)
		p.Require().Equal(p.header.Height, height)

		height, err = progress.ProcessedIndex()
		p.Require().NoError(err)
		p.Require().Equal(p.header.Height, height)
	})
}

func (p *PersisterSuite) TestPersister_ErrorHandling() {
	p.Run("persistor error", func() {
		expectedErr := errors.New("event persistor error")

		lockManager := storage.NewTestingLockManager()

		database := storagemock.NewDB(p.T())
		database.
			On("WithReaderBatchWriter", mock.Anything).
			Return(func(fn func(storage.ReaderBatchWriter) error) error {
				return fn(storagemock.NewBatch(p.T()))
			}).
			Once()

		collections := storagemock.NewCollections(p.T())
		collections.
			On("BatchStoreAndIndexByTransaction", mock.Anything, mock.Anything, mock.Anything).
			Return(nil, nil).
			Times(len(p.indexerData.Collections))

		events := storagemock.NewEvents(p.T())
		events.On("BatchStore", p.executionResult.BlockID, mock.Anything, mock.Anything).Return(expectedErr).Once()

		persister := NewBlockPersister(
			unittest.Logger(),
			database,
			lockManager,
			p.executionResult,
			[]stores.PersisterStore{
				stores.NewCollectionsStore(p.indexerData.Collections, collections, lockManager),
				stores.NewEventsStore(p.indexerData.Events, events, p.executionResult.BlockID),
			},
		)

		err := persister.Persist()
		p.Require().ErrorIs(err, expectedErr)
	})

	p.Run("lock manager error", func() {
		lockManager := lockctx.NewManager(nil, lockctx.NoPolicy)

		database := storagemock.NewDB(p.T())
		collections := storagemock.NewCollections(p.T())
		events := storagemock.NewEvents(p.T())

		persister := NewBlockPersister(
			unittest.Logger(),
			database,
			lockManager,
			p.executionResult,
			[]stores.PersisterStore{
				stores.NewCollectionsStore(p.indexerData.Collections, collections, lockManager),
				stores.NewEventsStore(p.indexerData.Events, events, p.executionResult.BlockID),
			},
		)

		err := persister.Persist()
		p.Require().Error(err)
		p.True(lockctx.IsUnknownLockError(err))
	})
}
