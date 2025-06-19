package indexer

import (
	"testing"

	"github.com/onflow/flow-go/storage"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/store/inmemory/unsynchronized"
	"github.com/onflow/flow-go/utils/unittest"
)

type PersisterSuite struct {
	suite.Suite
	persister              *BlockPersister
	inMemoryRegisters      *unsynchronized.Registers
	inMemoryEvents         *unsynchronized.Events
	inMemoryCollections    *unsynchronized.Collections
	inMemoryTransactions   *unsynchronized.Transactions
	inMemoryResults        *unsynchronized.LightTransactionResults
	inMemoryTxResultErrMsg *unsynchronized.TransactionResultErrorMessages
	registers              *storagemock.RegisterIndex
	events                 *storagemock.Events
	collections            *storagemock.Collections
	transactions           *storagemock.Transactions
	results                *storagemock.LightTransactionResults
	txResultErrMsg         *storagemock.TransactionResultErrorMessages
	database               *storagemock.DB
	executionResult        *flow.ExecutionResult
	header                 *flow.Header
}

func TestPersisterSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(PersisterSuite))
}

func (p *PersisterSuite) SetupTest() {
	t := p.T()

	block := unittest.BlockFixture()
	p.header = block.Header
	p.executionResult = unittest.ExecutionResultFixture(unittest.WithBlock(&block))

	p.inMemoryRegisters = unsynchronized.NewRegisters(p.header.Height)
	p.inMemoryEvents = unsynchronized.NewEvents()
	p.inMemoryCollections = unsynchronized.NewCollections()
	p.inMemoryTransactions = unsynchronized.NewTransactions()
	p.inMemoryResults = unsynchronized.NewLightTransactionResults()
	p.inMemoryTxResultErrMsg = unsynchronized.NewTransactionResultErrorMessages()

	p.registers = storagemock.NewRegisterIndex(t)
	p.events = storagemock.NewEvents(t)
	p.collections = storagemock.NewCollections(t)
	p.transactions = storagemock.NewTransactions(t)
	p.results = storagemock.NewLightTransactionResults(t)
	p.txResultErrMsg = storagemock.NewTransactionResultErrorMessages(t)

	p.database = storagemock.NewDB(t)
	p.database.On("WithReaderBatchWriter", mock.Anything).Return(
		func(fn func(storage.ReaderBatchWriter) error) error {
			return fn(storagemock.NewBatch(t))
		},
	)

	p.persister = NewPersister(
		zerolog.Nop(),
		p.inMemoryRegisters,
		p.inMemoryEvents,
		p.inMemoryCollections,
		p.inMemoryTransactions,
		p.inMemoryResults,
		p.inMemoryTxResultErrMsg,
		p.registers,
		p.events,
		p.collections,
		p.transactions,
		p.results,
		p.txResultErrMsg,
		p.database,
		p.executionResult,
		p.header,
	)
}

func (p *PersisterSuite) populateInMemoryStorages() {
	regEntries := make(flow.RegisterEntries, 3)
	for i := 0; i < 3; i++ {
		regEntries[i] = unittest.RegisterEntryFixture()
	}
	err := p.inMemoryRegisters.Store(regEntries, p.header.Height)
	p.Require().NoError(err)

	eventsList := unittest.EventsFixture(5)
	err = p.inMemoryEvents.Store(p.executionResult.BlockID, []flow.EventsList{eventsList})
	p.Require().NoError(err)

	for i := 0; i < 2; i++ {
		collection := unittest.CollectionFixture(2)
		light := collection.Light()
		err := p.inMemoryCollections.StoreLightAndIndexByTransaction(&light)
		p.Require().NoError(err)

		for _, tx := range collection.Transactions {
			err := p.inMemoryTransactions.Store(tx)
			p.Require().NoError(err)
		}
	}

	results := unittest.LightTransactionResultsFixture(4)
	err = p.inMemoryResults.Store(p.executionResult.BlockID, results)
	p.Require().NoError(err)

	txResultErrMsgs := make([]flow.TransactionResultErrorMessage, 2)
	executorID := unittest.IdentifierFixture()
	for i := 0; i < 2; i++ {
		txResultErrMsgs[i] = flow.TransactionResultErrorMessage{
			TransactionID: unittest.IdentifierFixture(),
			ErrorMessage:  "expected test error",
			Index:         uint32(i),
			ExecutorID:    executorID,
		}
	}
	err = p.inMemoryTxResultErrMsg.Store(p.executionResult.BlockID, txResultErrMsgs)
	p.Require().NoError(err)
}

func (p *PersisterSuite) TestPersister_PersistWithEmptyData() {
	t := p.T()

	err := p.inMemoryEvents.Store(p.executionResult.BlockID, []flow.EventsList{})
	p.Require().NoError(err)

	err = p.inMemoryResults.Store(p.executionResult.BlockID, []flow.LightTransactionResult{})
	p.Require().NoError(err)

	err = p.inMemoryTxResultErrMsg.Store(p.executionResult.BlockID, []flow.TransactionResultErrorMessage{})
	p.Require().NoError(err)

	storedRegisters := make([]flow.RegisterEntry, 0)
	p.registers.On("Store", mock.Anything, p.header.Height).Run(func(args mock.Arguments) {
		sr, ok := args.Get(0).(flow.RegisterEntries)
		p.Require().True(ok)
		storedRegisters = sr
	}).Return(nil).Once()

	err = p.persister.Persist()
	p.Require().NoError(err)

	// Verify registers were stored (even if empty)
	p.Assert().Empty(storedRegisters)

	// Verify other storages were not called since the data is empty
	p.events.AssertNotCalled(t, "BatchStore")
	p.results.AssertNotCalled(t, "BatchStore")
	p.collections.AssertNotCalled(t, "BatchStoreLightAndIndexByTransaction")
	p.transactions.AssertNotCalled(t, "BatchStore")
	p.txResultErrMsg.AssertNotCalled(t, "BatchStore")
}

func (p *PersisterSuite) TestPersister_PersistWithData() {
	p.populateInMemoryStorages()

	storedRegisters := make([]flow.RegisterEntry, 0)
	storedEvents := make([]flow.EventsList, 0)
	storedCollections := make([]flow.LightCollection, 0)
	storedTransactions := make([]flow.TransactionBody, 0)
	storedResults := make([]flow.LightTransactionResult, 0)
	storedTxResultErrMsgs := make([]flow.TransactionResultErrorMessage, 0)

	p.registers.On("Store", mock.Anything, p.header.Height).Run(func(args mock.Arguments) {
		sr, ok := args.Get(0).(flow.RegisterEntries)
		p.Require().True(ok)
		storedRegisters = sr
	}).Return(nil)

	p.events.On("BatchStore", p.executionResult.BlockID, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		se, ok := args.Get(1).([]flow.EventsList)
		p.Require().True(ok)
		storedEvents = se
	}).Return(nil)

	p.collections.On("BatchStoreLightAndIndexByTransaction", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		collection, ok := args.Get(0).(*flow.LightCollection)
		p.Require().True(ok)
		storedCollections = append(storedCollections, *collection)
	}).Return(nil)

	p.transactions.On("BatchStore", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		transaction, ok := args.Get(0).(*flow.TransactionBody)
		p.Require().True(ok)
		storedTransactions = append(storedTransactions, *transaction)
	}).Return(nil)

	p.results.On("BatchStore", p.executionResult.BlockID, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		sr, ok := args.Get(1).([]flow.LightTransactionResult)
		p.Require().True(ok)
		storedResults = sr
	}).Return(nil)

	p.txResultErrMsg.On("BatchStore", p.executionResult.BlockID, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		terrm, ok := args.Get(1).([]flow.TransactionResultErrorMessage)
		p.Require().True(ok)
		storedTxResultErrMsgs = terrm
	}).Return(nil)

	err := p.persister.Persist()
	p.Require().NoError(err)

	// Verify all mocks were called as expected
	p.Assert().ElementsMatch([]flow.EventsList{p.inMemoryEvents.Data()}, storedEvents)
	p.Assert().ElementsMatch(p.inMemoryResults.Data(), storedResults)

	registers, err := p.inMemoryRegisters.Data(p.header.Height)
	p.Require().NoError(err)
	p.Assert().ElementsMatch(registers, storedRegisters)

	p.Assert().ElementsMatch(p.inMemoryCollections.LightCollections(), storedCollections)
	p.Assert().ElementsMatch(p.inMemoryTransactions.Data(), storedTransactions)
	p.Assert().ElementsMatch(p.inMemoryTxResultErrMsg.Data(), storedTxResultErrMsgs)
}

func (p *PersisterSuite) TestPersister_PersistErrorHandling() {
	tests := []struct {
		name          string
		setupMocks    func()
		expectedError string
	}{
		{
			name: "RegistersStoreError",
			setupMocks: func() {
				p.registers.On("Store", mock.Anything, p.header.Height).Return(assert.AnError).Once()
			},
			expectedError: "could not persist registers",
		},
		{
			name: "EventsBatchStoreError",
			setupMocks: func() {
				p.registers.On("Store", mock.Anything, p.header.Height).Return(nil).Once()
				p.events.On("BatchStore", p.executionResult.BlockID, mock.Anything, mock.Anything).Return(assert.AnError).Once()
			},
			expectedError: "could not add events to batch",
		},
		{
			name: "ResultsBatchStoreError",
			setupMocks: func() {
				p.registers.On("Store", mock.Anything, p.header.Height).Return(nil).Once()
				p.events.On("BatchStore", p.executionResult.BlockID, mock.Anything, mock.Anything).Return(nil).Once()
				p.results.On("BatchStore", p.executionResult.BlockID, mock.Anything, mock.Anything).Return(assert.AnError).Once()
			},
			expectedError: "could not add transaction results to batch",
		},
		{
			name: "CollectionsStoreError",
			setupMocks: func() {
				p.registers.On("Store", mock.Anything, p.header.Height).Return(nil).Once()
				p.events.On("BatchStore", p.executionResult.BlockID, mock.Anything, mock.Anything).Return(nil).Once()
				p.results.On("BatchStore", p.executionResult.BlockID, mock.Anything, mock.Anything).Return(nil).Once()
				p.collections.On("BatchStoreLightAndIndexByTransaction", mock.Anything, mock.Anything).Return(assert.AnError).Once()
			},
			expectedError: "could not add collections to batch",
		},
		{
			name: "TransactionsStoreError",
			setupMocks: func() {
				p.registers.On("Store", mock.Anything, p.header.Height).Return(nil).Once()
				p.events.On("BatchStore", p.executionResult.BlockID, mock.Anything, mock.Anything).Return(nil).Once()
				p.results.On("BatchStore", p.executionResult.BlockID, mock.Anything, mock.Anything).Return(nil).Once()
				numberOfCollections := len(p.inMemoryCollections.LightCollections())
				p.collections.On("BatchStoreLightAndIndexByTransaction", mock.Anything, mock.Anything).Return(nil).Times(numberOfCollections)
				p.transactions.On("BatchStore", mock.Anything, mock.Anything).Return(assert.AnError).Once()
			},
			expectedError: "could not add transactions to batch",
		},
		{
			name: "TxResultErrMsgStoreError",
			setupMocks: func() {
				p.registers.On("Store", mock.Anything, p.header.Height).Return(nil).Once()
				p.events.On("BatchStore", p.executionResult.BlockID, mock.Anything, mock.Anything).Return(nil).Once()
				p.results.On("BatchStore", p.executionResult.BlockID, mock.Anything, mock.Anything).Return(nil).Once()
				numberOfCollections := len(p.inMemoryCollections.LightCollections())
				p.collections.On("BatchStoreLightAndIndexByTransaction", mock.Anything, mock.Anything).Return(nil).Times(numberOfCollections)
				numberOfTransactions := len(p.inMemoryTransactions.Data())
				p.transactions.On("BatchStore", mock.Anything, mock.Anything).Return(nil).Times(numberOfTransactions)
				p.txResultErrMsg.On("BatchStore", p.executionResult.BlockID, mock.Anything, mock.Anything).Return(assert.AnError).Once()
			},
			expectedError: "could not add transaction result error messages to batch",
		},
	}

	p.populateInMemoryStorages()

	for _, test := range tests {
		p.Run(test.name, func() {
			test.setupMocks()

			err := p.persister.Persist()
			p.Require().Error(err)

			p.Assert().Contains(err.Error(), test.expectedError)
		})
	}
}
