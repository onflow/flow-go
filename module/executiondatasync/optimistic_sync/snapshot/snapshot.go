package snapshot

import (
	execdatacache "github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/storage"
)

type Mock struct {
	events                         storage.EventsReader
	collections                    storage.CollectionsReader
	transactions                   storage.TransactionsReader
	lightTransactionResults        storage.LightTransactionResultsReader
	transactionResultErrorMessages storage.TransactionResultErrorMessagesReader
	registers                      storage.RegisterIndexReader
	executionData                  execdatacache.ExecutionDataCache
}

var _ optimistic_sync.Snapshot = (*Mock)(nil)

func NewSnapshotMock(
	events storage.EventsReader,
	collections storage.CollectionsReader,
	transactions storage.TransactionsReader,
	lightTransactionResults storage.LightTransactionResultsReader,
	transactionResultErrorMessages storage.TransactionResultErrorMessagesReader,
	registers storage.RegisterIndexReader,
	executionData execdatacache.ExecutionDataCache,
) *Mock {
	return &Mock{
		events:                         events,
		collections:                    collections,
		transactions:                   transactions,
		lightTransactionResults:        lightTransactionResults,
		transactionResultErrorMessages: transactionResultErrorMessages,
		registers:                      registers,
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

func (s *Mock) Registers() storage.RegisterIndexReader {
	return s.registers
}

func (s *Mock) ExecutionData() execdatacache.ExecutionDataCache { return s.executionData }
