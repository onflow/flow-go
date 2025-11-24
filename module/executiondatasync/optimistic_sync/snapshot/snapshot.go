package snapshot

import (
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/storage"
)

type Mock struct {
	events                         storage.EventsReader
	collections                    storage.CollectionsReader
	transactions                   storage.TransactionsReader
	lightTransactionResults        storage.LightTransactionResultsReader
	transactionResultErrorMessages storage.TransactionResultErrorMessagesReader
	registersAsyncStore            *execution.RegistersAsyncStore
	executionData                  optimistic_sync.BlockExecutionDataReader
}

var _ optimistic_sync.Snapshot = (*Mock)(nil)

func NewSnapshotMock(
	events storage.EventsReader,
	collections storage.CollectionsReader,
	transactions storage.TransactionsReader,
	lightTransactionResults storage.LightTransactionResultsReader,
	transactionResultErrorMessages storage.TransactionResultErrorMessagesReader,
	registersAsyncStore *execution.RegistersAsyncStore,
	executionData optimistic_sync.BlockExecutionDataReader,
) *Mock {
	return &Mock{
		events:                         events,
		collections:                    collections,
		transactions:                   transactions,
		lightTransactionResults:        lightTransactionResults,
		transactionResultErrorMessages: transactionResultErrorMessages,
		registersAsyncStore:            registersAsyncStore,
		executionData:                  executionData,
	}
}

func (s *Mock) Events() storage.EventsReader {
	return s.events
}

func (s *Mock) Collections() storage.CollectionsReader {
	return s.collections
}

func (s *Mock) Transactions() storage.TransactionsReader {
	return s.transactions
}

func (s *Mock) LightTransactionResults() storage.LightTransactionResultsReader {
	return s.lightTransactionResults
}

func (s *Mock) TransactionResultErrorMessages() storage.TransactionResultErrorMessagesReader {
	return s.transactionResultErrorMessages
}

// Registers returns a reader for querying register data.
//
// Expected error returns during normal operation:
//   - [indexer.ErrIndexNotInitialized]: If the storage is still bootstrapping.
func (s *Mock) Registers() (storage.RegisterSnapshotReader, error) {
	return s.registersAsyncStore.RegisterSnapshotReader()
}

func (s *Mock) BlockExecutionData() optimistic_sync.BlockExecutionDataReader { return s.executionData }
