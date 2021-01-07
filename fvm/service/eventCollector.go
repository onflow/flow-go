package service

import (
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-go/model/flow"
)

type EventCollector struct {
	events             []flow.Event
	totalEventByteSize uint64
}

func NewEventCollector() EventCollector {
	return EventCollector{}
}

func (e *EventCollector) EmitEvent(event cadence.Event) error {
	payload, err := jsoncdc.Encode(event)
	if err != nil {
		return fmt.Errorf("failed to json encode a cadence event: %w", err)
	}

	e.totalEventByteSize += uint64(len(payload))

	// skip limit if payer is service account
	if e.transactionEnv.tx.Payer != e.ctx.Chain.ServiceAddress() {
		if e.totalEventByteSize > e.ctx.EventCollectionByteSizeLimit {
			return &EventLimitExceededError{
				TotalByteSize: e.totalEventByteSize,
				Limit:         e.ctx.EventCollectionByteSizeLimit,
			}
		}
	}

	flowEvent := flow.Event{
		Type:             flow.EventType(event.EventType.ID()),
		TransactionID:    e.transactionEnv.TxID(),
		TransactionIndex: e.transactionEnv.TxIndex(),
		EventIndex:       uint32(len(e.events)),
		Payload:          payload,
	}

	e.events = append(e.events, flowEvent)
	return nil
}
