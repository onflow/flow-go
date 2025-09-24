package snapshot

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/storage"
)

type Mock struct {
	events                         storage.EventsReader
	lightTransactionResults        storage.LightTransactionResultsReader
	transactionResultErrorMessages storage.TransactionResultErrorMessagesReader
	registers                      storage.RegisterIndexReader
	blockStatus                    flow.BlockStatus
}

var _ optimistic_sync.Snapshot = (*Mock)(nil)

func NewSnapshotMock(
	events storage.EventsReader,
	lightTransactionResults storage.LightTransactionResultsReader,
	transactionResultErrorMessages storage.TransactionResultErrorMessagesReader,
	registers storage.RegisterIndexReader,
	blockStatus flow.BlockStatus,
) *Mock {
	return &Mock{
		events:                         events,
		lightTransactionResults:        lightTransactionResults,
		transactionResultErrorMessages: transactionResultErrorMessages,
		registers:                      registers,
		blockStatus:                    blockStatus,
	}
}

func (s *Mock) Events() storage.EventsReader {
	return s.events
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

func (s *Mock) BlockStatus() flow.BlockStatus {
	return s.blockStatus
}
