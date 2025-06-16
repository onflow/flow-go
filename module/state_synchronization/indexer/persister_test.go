package indexer

import (
	"testing"

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
	persister              *Persister
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
	batch                  *storagemock.Batch
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

	p.header = unittest.BlockHeaderFixture()
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

	p.batch = storagemock.NewBatch(t)
	p.batch.On("Commit").Return(nil).Maybe()
	p.batch.On("Close").Return(nil)
	p.database = storagemock.NewDB(t)
	p.database.On("NewBatch").Return(p.batch)

	block := unittest.BlockWithParentFixture(p.header)
	p.executionResult = unittest.ExecutionResultFixture(unittest.WithBlock(block))

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

	p.populateInMemoryStorages()
}

func (p *PersisterSuite) populateInMemoryStorages() {
	regEntries := make(flow.RegisterEntries, 3)
	for i := 0; i < 3; i++ {
		regEntries[i] = unittest.RegisterEntryFixture()
	}
	p.Require().NoError(p.inMemoryRegisters.Store(regEntries, p.header.Height))

	eventsList := unittest.EventsFixture(5)
	err := p.inMemoryEvents.Store(p.executionResult.BlockID, []flow.EventsList{eventsList})
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
	err = p.inMemoryTxResultErrMsg.Store(p.header.ID(), txResultErrMsgs)
	p.Require().NoError(err)
}

func (p *PersisterSuite) TestPersister_AddToBatch_WithData() {
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

	p.events.On("BatchStore", p.executionResult.BlockID, mock.Anything, p.batch).Run(func(args mock.Arguments) {
		se, ok := args.Get(1).([]flow.EventsList)
		p.Require().True(ok)
		storedEvents = se
	}).Return(nil)

	p.collections.On("BatchStoreLightAndIndexByTransaction", mock.Anything, p.batch).Run(func(args mock.Arguments) {
		collection, ok := args.Get(0).(*flow.LightCollection)
		p.Require().True(ok)
		storedCollections = append(storedCollections, *collection)
	}).Return(nil)

	p.transactions.On("BatchStore", mock.Anything, p.batch).Run(func(args mock.Arguments) {
		transaction, ok := args.Get(0).(*flow.TransactionBody)
		p.Require().True(ok)
		storedTransactions = append(storedTransactions, *transaction)
	}).Return(nil)

	p.results.On("BatchStore", p.executionResult.BlockID, mock.Anything, p.batch).Run(func(args mock.Arguments) {
		sr, ok := args.Get(1).([]flow.LightTransactionResult)
		p.Require().True(ok)
		storedResults = sr
	}).Return(nil)

	p.txResultErrMsg.On("BatchStore", p.header.ID(), mock.Anything, p.batch).Run(func(args mock.Arguments) {
		terrm, ok := args.Get(1).([]flow.TransactionResultErrorMessage)
		p.Require().True(ok)
		storedTxResultErrMsgs = terrm
	}).Return(nil)

	err := p.persister.Persist()
	p.Assert().NoError(err)

	// Verify all mocks were called as expected
	p.events.AssertExpectations(p.T())
	p.Assert().ElementsMatch([]flow.EventsList{p.inMemoryEvents.Data()}, storedEvents)

	p.results.AssertExpectations(p.T())
	p.Assert().ElementsMatch(p.inMemoryResults.Data(), storedResults)

	p.registers.AssertExpectations(p.T())
	p.Assert().ElementsMatch(p.inMemoryRegisters.Data(), storedRegisters)

	p.collections.AssertExpectations(p.T())
	p.Assert().ElementsMatch(p.inMemoryCollections.LightCollections(), storedCollections)

	p.transactions.AssertExpectations(p.T())
	p.Assert().ElementsMatch(p.inMemoryTransactions.Data(), storedTransactions)

	p.txResultErrMsg.AssertExpectations(p.T())
	p.Assert().ElementsMatch(p.inMemoryTxResultErrMsg.Data(), storedTxResultErrMsgs)
}

func (p *PersisterSuite) TestPersister_Persist_ErrorHandling() {
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
				p.events.On("BatchStore", p.executionResult.BlockID, mock.Anything, p.batch).Return(assert.AnError).Once()
			},
			expectedError: "could not add events to batch",
		},
		{
			name: "ResultsBatchStoreError",
			setupMocks: func() {
				p.registers.On("Store", mock.Anything, p.header.Height).Return(nil).Once()
				p.events.On("BatchStore", p.executionResult.BlockID, mock.Anything, p.batch).Return(nil).Once()
				p.results.On("BatchStore", p.executionResult.BlockID, mock.Anything, p.batch).Return(assert.AnError).Once()
			},
			expectedError: "could not add transaction results to batch",
		},
		{
			name: "CollectionsStoreError",
			setupMocks: func() {
				p.registers.On("Store", mock.Anything, p.header.Height).Return(nil).Once()
				p.events.On("BatchStore", p.executionResult.BlockID, mock.Anything, p.batch).Return(nil).Once()
				p.results.On("BatchStore", p.executionResult.BlockID, mock.Anything, p.batch).Return(nil).Once()
				p.collections.On("BatchStoreLightAndIndexByTransaction", mock.Anything, mock.Anything).Return(assert.AnError).Once()
			},
			expectedError: "could not add collections to batch",
		},
		{
			name: "TransactionsStoreError",
			setupMocks: func() {
				p.registers.On("Store", mock.Anything, p.header.Height).Return(nil).Once()
				p.events.On("BatchStore", p.executionResult.BlockID, mock.Anything, p.batch).Return(nil).Once()
				p.results.On("BatchStore", p.executionResult.BlockID, mock.Anything, p.batch).Return(nil).Once()
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
				p.events.On("BatchStore", p.executionResult.BlockID, mock.Anything, p.batch).Return(nil).Once()
				p.results.On("BatchStore", p.executionResult.BlockID, mock.Anything, p.batch).Return(nil).Once()
				numberOfCollections := len(p.inMemoryCollections.LightCollections())
				p.collections.On("BatchStoreLightAndIndexByTransaction", mock.Anything, mock.Anything).Return(nil).Times(numberOfCollections)
				numberOfTransactions := len(p.inMemoryTransactions.Data())
				p.transactions.On("BatchStore", mock.Anything, mock.Anything).Return(nil).Times(numberOfTransactions)
				p.txResultErrMsg.On("BatchStore", p.header.ID(), mock.Anything, p.batch).Return(assert.AnError).Once()
			},
			expectedError: "could not add transaction result error messages to batch",
		},
	}

	for _, test := range tests {
		p.Run(test.name, func() {
			test.setupMocks()

			err := p.persister.Persist()

			p.Assert().Error(err)
			p.Assert().Contains(err.Error(), test.expectedError)
		})
	}
}
