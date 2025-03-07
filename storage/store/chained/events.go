package chained

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type ChainedEvents struct {
	first  storage.EventsReader
	second storage.EventsReader
}

var _ storage.EventsReader = (*ChainedEvents)(nil)

// NewEvents returns a new ChainedEvents events store, which will handle reads. Any writes query
// will return err
// for reads, it first query first database, then second database, this is useful when migrating
// data from badger to pebble
func NewEvents(first storage.EventsReader, second storage.EventsReader) *ChainedEvents {
	return &ChainedEvents{
		first:  first,
		second: second,
	}
}

func (c *ChainedEvents) ByBlockID(blockID flow.Identifier) ([]flow.Event, error) {
	events, err := c.first.ByBlockID(blockID)
	if err != nil {
		return nil, err
	}

	if len(events) > 0 {
		return events, nil
	}

	return c.second.ByBlockID(blockID)
}

func (c *ChainedEvents) ByBlockIDTransactionID(blockID flow.Identifier, transactionID flow.Identifier) ([]flow.Event, error) {
	events, err := c.first.ByBlockIDTransactionID(blockID, transactionID)
	if err != nil {
		return nil, err
	}

	if len(events) > 0 {
		return events, nil
	}

	return c.second.ByBlockIDTransactionID(blockID, transactionID)
}

func (c *ChainedEvents) ByBlockIDTransactionIndex(blockID flow.Identifier, txIndex uint32) ([]flow.Event, error) {
	events, err := c.first.ByBlockIDTransactionIndex(blockID, txIndex)
	if err != nil {
		return nil, err
	}

	if len(events) > 0 {
		return events, nil
	}

	return c.second.ByBlockIDTransactionIndex(blockID, txIndex)
}

func (c *ChainedEvents) ByBlockIDEventType(blockID flow.Identifier, eventType flow.EventType) ([]flow.Event, error) {
	events, err := c.first.ByBlockIDEventType(blockID, eventType)
	if err != nil {
		return nil, err
	}

	if len(events) > 0 {
		return events, nil
	}

	return c.second.ByBlockIDEventType(blockID, eventType)
}
