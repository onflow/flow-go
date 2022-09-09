package environment

import (
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

// EventEmitter collect events, separates out service events, and enforces
// event size limits.
//
// Note that scripts do not emit events, but must expose the API in compliance
// with the runtime environment interface.
type EventEmitter interface {
	// Cadence's runtime API.  Note that the script variant will return
	// OperationNotSupportedError.
	EmitEvent(event cadence.Event) error

	Events() []flow.Event
	ServiceEvents() []flow.Event
}

var _ EventEmitter = NoEventEmitter{}

// NoEventEmitter is usually used in the environment for script execution,
// where emitting an event does nothing.
type NoEventEmitter struct{}

func (NoEventEmitter) EmitEvent(event cadence.Event) error {
	return nil
}

func (NoEventEmitter) Events() []flow.Event {
	return []flow.Event{}
}

func (NoEventEmitter) ServiceEvents() []flow.Event {
	return []flow.Event{}
}

type eventEmitter struct {
	tracer *Tracer
	meter  Meter

	chain   flow.Chain
	txID    flow.Identifier
	txIndex uint32
	payer   flow.Address

	serviceEventCollectionEnabled bool
	eventCollectionByteSizeLimit  uint64
	eventCollection               *EventCollection
}

// NewEventEmitter constructs a new eventEmitter
func NewEventEmitter(
	tracer *Tracer,
	meter Meter,
	chain flow.Chain,
	txID flow.Identifier,
	txIndex uint32,
	payer flow.Address,
	serviceEventCollectionEnabled bool,
	eventCollectionByteSizeLimit uint64,
) EventEmitter {
	return &eventEmitter{
		tracer:                        tracer,
		meter:                         meter,
		chain:                         chain,
		txID:                          txID,
		txIndex:                       txIndex,
		payer:                         payer,
		serviceEventCollectionEnabled: serviceEventCollectionEnabled,
		eventCollectionByteSizeLimit:  eventCollectionByteSizeLimit,
		eventCollection:               NewEventCollection(),
	}
}

func (emitter *eventEmitter) EventCollection() *EventCollection {
	return emitter.eventCollection
}

func (emitter *eventEmitter) EmitEvent(event cadence.Event) error {
	defer emitter.tracer.StartExtensiveTracingSpanFromRoot(
		trace.FVMEnvEmitEvent).End()

	err := emitter.meter.MeterComputation(ComputationKindEmitEvent, 1)
	if err != nil {
		return fmt.Errorf("emit event failed: %w", err)
	}

	payload, err := jsoncdc.Encode(event)
	if err != nil {
		return errors.NewEncodingFailuref(
			"failed to json encode a cadence event: %w",
			err)
	}

	payloadSize := uint64(len(payload))

	// skip limit if payer is service account
	if emitter.payer != emitter.chain.ServiceAddress() {
		totalSize := emitter.eventCollection.TotalByteSize() + payloadSize
		if totalSize > emitter.eventCollectionByteSizeLimit {
			return errors.NewEventLimitExceededError(
				totalSize,
				emitter.eventCollectionByteSizeLimit)
		}
	}

	flowEvent := flow.Event{
		Type:             flow.EventType(event.EventType.ID()),
		TransactionID:    emitter.txID,
		TransactionIndex: emitter.txIndex,
		EventIndex:       emitter.eventCollection.eventCounter,
		Payload:          payload,
	}

	if emitter.serviceEventCollectionEnabled {
		ok, err := IsServiceEvent(event, emitter.chain.ChainID())
		if err != nil {
			return fmt.Errorf("unable to check service event: %w", err)
		}
		if ok {
			emitter.eventCollection.AppendServiceEvent(flowEvent, payloadSize)
		}
		// We don't return and append the service event into event collection
		// as well.
	}

	emitter.eventCollection.AppendEvent(flowEvent, payloadSize)
	return nil
}

func (emitter *eventEmitter) Events() []flow.Event {
	return emitter.eventCollection.events
}

func (emitter *eventEmitter) ServiceEvents() []flow.Event {
	return emitter.eventCollection.serviceEvents
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

func (collection *EventCollection) Events() []flow.Event {
	return collection.events
}

func (collection *EventCollection) AppendEvent(event flow.Event, size uint64) {
	collection.events = append(collection.events, event)
	collection.totalByteSize += size
	collection.eventCounter++
}

func (collection *EventCollection) ServiceEvents() []flow.Event {
	return collection.serviceEvents
}

func (collection *EventCollection) AppendServiceEvent(
	event flow.Event,
	size uint64,
) {
	collection.serviceEvents = append(collection.serviceEvents, event)
	collection.totalByteSize += size
	collection.eventCounter++
}

func (collection *EventCollection) TotalByteSize() uint64 {
	return collection.totalByteSize
}

// IsServiceEvent determines whether or not an emitted Cadence event is
// considered a service event for the given chain.
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
