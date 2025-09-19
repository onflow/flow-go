package snapshot

import (
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/storage"
)

type Mock struct {
	events    storage.EventsReader
	registers storage.RegisterIndexReader
}

var _ optimistic_sync.Snapshot = (*Mock)(nil)

func NewSnapshotMock(events storage.EventsReader, registers storage.RegisterIndexReader) *Mock {
	return &Mock{
		events:    events,
		registers: registers,
	}
}

func (s *Mock) Events() storage.EventsReader {
	return s.events
}

func (s *Mock) Collections() storage.CollectionsReader {
	//TODO implement me
	panic("implement me")
}

func (s *Mock) Transactions() storage.TransactionsReader {
	//TODO implement me
	panic("implement me")
}

func (s *Mock) LightTransactionResults() storage.LightTransactionResultsReader {
	//TODO implement me
	panic("implement me")
}

func (s *Mock) TransactionResultErrorMessages() storage.TransactionResultErrorMessagesReader {
	//TODO implement me
	panic("implement me")
}

func (s *Mock) Registers() storage.RegisterIndexReader {
	return s.registers
}
