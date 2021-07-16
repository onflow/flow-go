package handler

import (
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

// EventHandler collect events, separates out service events, and enforces event size limits
type EventHandler struct {
	chain                         flow.Chain
	eventCollectionEnabled        bool
	serviceEventCollectionEnabled bool
	eventCollectionByteSizeLimit  uint64
	eventCollection               *EventCollection
}

// NewEventHandler constructs a new EventHandler
func NewEventHandler(chain flow.Chain,
	eventCollectionEnabled bool,
	serviceEventCollectionEnabled bool,
	eventCollectionByteSizeLimit uint64) *EventHandler {
	return &EventHandler{
		chain:                         chain,
		eventCollectionEnabled:        eventCollectionEnabled,
		serviceEventCollectionEnabled: serviceEventCollectionEnabled,
		eventCollectionByteSizeLimit:  eventCollectionByteSizeLimit,
		eventCollection:               NewEventCollection(),
	}
}

func (h *EventHandler) EventCollection() *EventCollection {
	return h.eventCollection
}

func (h *EventHandler) EmitEvent(event cadence.Event,
	txID flow.Identifier,
	txIndex uint32,
	payer flow.Address) error {
	if !h.eventCollectionEnabled {
		return nil
	}

	payload, err := jsoncdc.Encode(event)
	if err != nil {
		return errors.NewEncodingFailuref("failed to json encode a cadence event: %w", err)
	}

	payloadSize := uint64(len(payload))

	// skip limit if payer is service account
	if payer != h.chain.ServiceAddress() {
		if h.eventCollection.TotalByteSize()+payloadSize > h.eventCollectionByteSizeLimit {
			return errors.NewEventLimitExceededError(h.eventCollection.TotalByteSize()+payloadSize, h.eventCollectionByteSizeLimit)
		}
	}

	flowEvent := flow.Event{
		Type:             flow.EventType(event.EventType.ID()),
		TransactionID:    txID,
		TransactionIndex: txIndex,
		EventIndex:       h.eventCollection.eventCounter,
		Payload:          payload,
	}

	if h.serviceEventCollectionEnabled {
		ok, err := IsServiceEvent(event, h.chain.ChainID())
		if err != nil {
			return fmt.Errorf("unable to check service event: %w", err)
		}
		if ok {
			h.eventCollection.AppendServiceEvent(flowEvent, payloadSize)
		}
		// we don't return and append the service event into event collection as well.
	}

	h.eventCollection.AppendEvent(flowEvent, payloadSize)
	return nil
}

func (h *EventHandler) Events() []flow.Event {
	return h.eventCollection.events
}

func (h *EventHandler) ServiceEvents() []flow.Event {
	return h.eventCollection.serviceEvents
}

type EventCollection struct {
	events        flow.EventsList
	serviceEvents flow.EventsList
	totalByteSize uint64
	eventCounter  uint32
}

func NewEventCollection() *EventCollection {
	return &EventCollection{
		events:        make([]flow.Event, 0, 10),
		serviceEvents: make([]flow.Event, 0, 10),
		totalByteSize: uint64(0),
		eventCounter:  uint32(0),
	}
}

func (e *EventCollection) Child() *EventCollection {
	res := NewEventCollection()
	res.eventCounter = e.eventCounter
	return res
}

// Merge merges another event collection into this event collection
func (e *EventCollection) Merge(other *EventCollection) {
	e.events = append(e.events, other.events...)
	e.serviceEvents = append(e.serviceEvents, other.serviceEvents...)
	e.totalByteSize = e.totalByteSize + other.totalByteSize
	e.eventCounter = e.eventCounter + other.eventCounter
}

func (e *EventCollection) Events() []flow.Event {
	return e.events
}

func (e *EventCollection) AppendEvent(event flow.Event, size uint64) {
	e.events = append(e.events, event)
	e.totalByteSize += size
	e.eventCounter++
}

func (e *EventCollection) ServiceEvents() []flow.Event {
	return e.serviceEvents
}

func (e *EventCollection) AppendServiceEvent(event flow.Event, size uint64) {
	e.serviceEvents = append(e.serviceEvents, event)
	e.totalByteSize += size
	e.eventCounter++
}

func (e *EventCollection) TotalByteSize() uint64 {
	return e.totalByteSize
}

// IsServiceEvent determines whether or not an emitted Cadence event is considered
// a service event for the given chain.
func IsServiceEvent(event cadence.Event, chain flow.ChainID) (bool, error) {

	// retrieve the service event information for this chain
	events, err := systemcontracts.ServiceEventsForChain(chain)
	if err != nil {
		return false, fmt.Errorf("unknown system contracts for chain (%s): %w", chain.String(), err)
	}

	eventType := flow.EventType(event.EventType.ID())
	for _, serviceEvent := range events.All() {
		if serviceEvent.EventType() == eventType {
			return true, nil
		}
	}

	return false, nil
}
