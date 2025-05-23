package indexer

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/store/inmemory/unsynchronized"
	"github.com/onflow/flow-go/utils/unittest"
)

type persisterTest struct {
	t                    *testing.T
	persister            *Persister
	inMemoryRegisters    *unsynchronized.Registers
	inMemoryEvents       *unsynchronized.Events
	inMemoryCollections  *unsynchronized.Collections
	inMemoryTransactions *unsynchronized.Transactions
	inMemoryResults      *unsynchronized.LightTransactionResults
	registers            *storagemock.RegisterIndex
	events               *storagemock.Events
	collections          *storagemock.Collections
	transactions         *storagemock.Transactions
	results              *storagemock.LightTransactionResults
	batch                *storagemock.Batch
	executionResult      *flow.ExecutionResult
	header               *flow.Header
}

func newPersisterTest(t *testing.T) *persisterTest {
	header := unittest.BlockHeaderFixture()
	block := unittest.BlockWithParentFixture(header)
	executionResult := unittest.ExecutionResultFixture(unittest.WithBlock(block))

	return &persisterTest{
		t:                    t,
		inMemoryRegisters:    unsynchronized.NewRegisters(header.Height),
		inMemoryEvents:       unsynchronized.NewEvents(),
		inMemoryCollections:  unsynchronized.NewCollections(),
		inMemoryTransactions: unsynchronized.NewTransactions(),
		inMemoryResults:      unsynchronized.NewLightTransactionResults(),
		registers:            storagemock.NewRegisterIndex(t),
		events:               storagemock.NewEvents(t),
		collections:          storagemock.NewCollections(t),
		transactions:         storagemock.NewTransactions(t),
		results:              storagemock.NewLightTransactionResults(t),
		batch:                storagemock.NewBatch(t),
		executionResult:      executionResult,
		header:               header,
	}
}

func (pt *persisterTest) initPersister() *persisterTest {
	pt.persister = NewPersister(
		zerolog.Nop(),
		pt.inMemoryRegisters,
		pt.inMemoryEvents,
		pt.inMemoryCollections,
		pt.inMemoryTransactions,
		pt.inMemoryResults,
		pt.registers,
		pt.events,
		pt.collections,
		pt.transactions,
		pt.results,
		pt.executionResult,
		pt.header,
	)
	return pt
}

func (pt *persisterTest) populateInMemoryStorages() *persisterTest {
	regEntries := make(flow.RegisterEntries, 3)
	for i := 0; i < 3; i++ {
		regEntries[i] = unittest.RegisterEntryFixture()
	}
	require.NoError(pt.t, pt.inMemoryRegisters.Store(regEntries, pt.header.Height))

	eventsList := unittest.EventsFixture(5)
	err := pt.inMemoryEvents.Store(pt.executionResult.BlockID, []flow.EventsList{eventsList})
	require.NoError(pt.t, err)

	for i := 0; i < 2; i++ {
		collection := unittest.CollectionFixture(2)
		err := pt.inMemoryCollections.Store(&collection)
		require.NoError(pt.t, err)

		for _, tx := range collection.Transactions {
			err := pt.inMemoryTransactions.Store(tx)
			require.NoError(pt.t, err)
		}
	}

	results := unittest.LightTransactionResultsFixture(4)
	err = pt.inMemoryResults.Store(pt.executionResult.BlockID, results)
	require.NoError(pt.t, err)

	return pt
}

// verifySuccess verifies that the operation completed successfully with no errors
func (pt *persisterTest) verifySuccess(err error) {
	assert.NoError(pt.t, err)

	// Verify all mocks were called as expected
	pt.events.AssertExpectations(pt.t)
	pt.results.AssertExpectations(pt.t)
	pt.registers.AssertExpectations(pt.t)
	pt.collections.AssertExpectations(pt.t)
	pt.transactions.AssertExpectations(pt.t)

	// Verify LastPersistedSealedExecutionResult was updated
	assert.Equal(pt.t, pt.executionResult, pt.persister.LastPersistedSealedExecutionResult)
}

// verifyError verifies that the operation failed with the expected error message
func (pt *persisterTest) verifyError(err error, errorMessage string) {
	assert.Error(pt.t, err)
	assert.Contains(pt.t, err.Error(), errorMessage)

	// Verify all mocks were called as expected
	pt.events.AssertExpectations(pt.t)
	pt.results.AssertExpectations(pt.t)
	pt.registers.AssertExpectations(pt.t)
	pt.collections.AssertExpectations(pt.t)
	pt.transactions.AssertExpectations(pt.t)
}

func TestPersister_AddToBatch_EmptyStorages(t *testing.T) {
	pt := newPersisterTest(t).initPersister()

	// Test AddToBatch with empty storages
	err := pt.persister.AddToBatch(pt.batch)

	// Verify the function worked as expected
	pt.verifySuccess(err)
}

func TestPersister_AddToBatch_WithData(t *testing.T) {
	pt := newPersisterTest(t).initPersister().populateInMemoryStorages()

	// Set up expectations for populated storages
	pt.events.On("BatchStore", pt.executionResult.BlockID, mock.Anything, pt.batch).Return(nil)
	pt.results.On("BatchStore", pt.executionResult.BlockID, mock.Anything, pt.batch).Return(nil)
	pt.registers.On("Store", mock.Anything, pt.header.Height).Return(nil)
	pt.collections.On("StoreLightAndIndexByTransaction", mock.Anything).Return(nil)
	pt.transactions.On("Store", mock.Anything).Return(nil)

	err := pt.persister.AddToBatch(pt.batch)

	pt.verifySuccess(err)
}

func TestPersister_AddToBatch_ExistingCollections(t *testing.T) {
	pt := newPersisterTest(t).initPersister().populateInMemoryStorages()

	// Set up expectations including collections returning "already exists"
	pt.events.On("BatchStore", pt.executionResult.BlockID, mock.Anything, pt.batch).Return(nil)
	pt.results.On("BatchStore", pt.executionResult.BlockID, mock.Anything, pt.batch).Return(nil)
	pt.registers.On("Store", mock.Anything, pt.header.Height).Return(nil)
	pt.collections.On("StoreLightAndIndexByTransaction", mock.Anything).Return(storage.ErrAlreadyExists)
	pt.transactions.On("Store", mock.Anything).Return(nil)

	err := pt.persister.AddToBatch(pt.batch)

	pt.verifySuccess(err)
}

func TestPersister_AddToBatch_ExistingTransactions(t *testing.T) {
	pt := newPersisterTest(t).initPersister().populateInMemoryStorages()

	// Set up expectations including transactions returning "already exists"
	pt.events.On("BatchStore", pt.executionResult.BlockID, mock.Anything, pt.batch).Return(nil)
	pt.results.On("BatchStore", pt.executionResult.BlockID, mock.Anything, pt.batch).Return(nil)
	pt.registers.On("Store", mock.Anything, pt.header.Height).Return(nil)
	pt.collections.On("StoreLightAndIndexByTransaction", mock.Anything).Return(nil)
	pt.transactions.On("Store", mock.Anything).Return(storage.ErrAlreadyExists)

	err := pt.persister.AddToBatch(pt.batch)

	pt.verifySuccess(err)
}

func TestPersister_AddToBatch_ErrorHandling(t *testing.T) {
	t.Run("RegistersStoreError", func(t *testing.T) {
		pt := newPersisterTest(t).initPersister().populateInMemoryStorages()

		// Only set up the registers mock to return an error
		pt.events.On("BatchStore", pt.executionResult.BlockID, mock.Anything, pt.batch).Return(nil)
		pt.results.On("BatchStore", pt.executionResult.BlockID, mock.Anything, pt.batch).Return(nil)
		pt.registers.On("Store", mock.Anything, pt.header.Height).Return(assert.AnError)

		// No need to set up collections and transactions as we expect early return

		err := pt.persister.AddToBatch(pt.batch)

		// Verify the function worked as expected
		pt.verifyError(err, "could not persist registers")
	})

	t.Run("EventsBatchStoreError", func(t *testing.T) {
		pt := newPersisterTest(t).initPersister().populateInMemoryStorages()

		// Only set up the events mock to return an error
		pt.events.On("BatchStore", pt.executionResult.BlockID, mock.Anything, pt.batch).Return(assert.AnError)

		// No need to set up other mocks as we expect early return

		err := pt.persister.AddToBatch(pt.batch)

		// Verify the function worked as expected
		pt.verifyError(err, "could not add events to batch")
	})

	t.Run("ResultsBatchStoreError", func(t *testing.T) {
		pt := newPersisterTest(t).initPersister().populateInMemoryStorages()

		// Set up events to succeed but results to fail
		pt.events.On("BatchStore", pt.executionResult.BlockID, mock.Anything, pt.batch).Return(nil)
		pt.results.On("BatchStore", pt.executionResult.BlockID, mock.Anything, pt.batch).Return(assert.AnError)

		// No need to set up other mocks as we expect early return

		err := pt.persister.AddToBatch(pt.batch)

		// Verify the function worked as expected
		pt.verifyError(err, "could not add transaction results to batch")
	})

	t.Run("CollectionsStoreError", func(t *testing.T) {
		pt := newPersisterTest(t).initPersister().populateInMemoryStorages()

		// Set up everything before collections to succeed, but collections to fail
		pt.events.On("BatchStore", pt.executionResult.BlockID, mock.Anything, pt.batch).Return(nil)
		pt.results.On("BatchStore", pt.executionResult.BlockID, mock.Anything, pt.batch).Return(nil)
		pt.registers.On("Store", mock.Anything, pt.header.Height).Return(nil)
		pt.collections.On("StoreLightAndIndexByTransaction", mock.Anything).Return(assert.AnError)

		// No need to set up transactions as we expect early return

		err := pt.persister.AddToBatch(pt.batch)

		// Verify the function worked as expected
		pt.verifyError(err, "could not persist collection")
	})

	t.Run("TransactionsStoreError", func(t *testing.T) {
		pt := newPersisterTest(t).initPersister().populateInMemoryStorages()

		// Set up everything before transactions to succeed, but transactions to fail
		pt.events.On("BatchStore", pt.executionResult.BlockID, mock.Anything, pt.batch).Return(nil)
		pt.results.On("BatchStore", pt.executionResult.BlockID, mock.Anything, pt.batch).Return(nil)
		pt.registers.On("Store", mock.Anything, pt.header.Height).Return(nil)
		pt.collections.On("StoreLightAndIndexByTransaction", mock.Anything).Return(nil)
		pt.transactions.On("Store", mock.Anything).Return(assert.AnError)

		err := pt.persister.AddToBatch(pt.batch)

		// Verify the function worked as expected
		pt.verifyError(err, "could not persist transaction")
	})
}
