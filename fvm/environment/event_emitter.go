package environment

import (
	"fmt"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

const (
	DefaultEventCollectionByteSizeLimit = 256_000 // 256KB
)

type EventEmitterParams struct {
	ServiceEventCollectionEnabled bool
	EventCollectionByteSizeLimit  uint64
	EventEncoder                  EventEncoder
}

func DefaultEventEmitterParams() EventEmitterParams {
	return EventEmitterParams{
		ServiceEventCollectionEnabled: false,
		EventCollectionByteSizeLimit:  DefaultEventCollectionByteSizeLimit,
		EventEncoder:                  NewCadenceEventEncoder(),
	}
}

// EventEmitter collect events, separates out service events, and enforces
// event size limits.
//
// Note that scripts do not emit events, but must expose the API in compliance
// with the runtime environment interface.
type EventEmitter interface {
	// Cadence's runtime API.  Note that the script variant will return
	// OperationNotSupportedError.
	EmitEvent(event cadence.Event) error

	Events() flow.EventsList
	ServiceEvents() flow.EventsList
	ConvertedServiceEvents() flow.ServiceEventList

	Reset()
}

type ParseRestrictedEventEmitter struct {
	txnState state.NestedTransaction
	impl     EventEmitter
}

func NewParseRestrictedEventEmitter(
	txnState state.NestedTransaction,
	impl EventEmitter,
) EventEmitter {
	return ParseRestrictedEventEmitter{
		txnState: txnState,
		impl:     impl,
	}
}

func (emitter ParseRestrictedEventEmitter) EmitEvent(event cadence.Event) error {
	return parseRestrict1Arg(
		emitter.txnState,
		trace.FVMEnvEmitEvent,
		emitter.impl.EmitEvent,
		event)
}

func (emitter ParseRestrictedEventEmitter) Events() flow.EventsList {
	return emitter.impl.Events()
}

func (emitter ParseRestrictedEventEmitter) ServiceEvents() flow.EventsList {
	return emitter.impl.ServiceEvents()
}

func (emitter ParseRestrictedEventEmitter) ConvertedServiceEvents() flow.ServiceEventList {
	return emitter.impl.ConvertedServiceEvents()
}

func (emitter ParseRestrictedEventEmitter) Reset() {
	emitter.impl.Reset()
}

var _ EventEmitter = NoEventEmitter{}

// NoEventEmitter is usually used in the environment for script execution,
// where emitting an event does nothing.
type NoEventEmitter struct{}

func (NoEventEmitter) EmitEvent(event cadence.Event) error {
	return nil
}

func (NoEventEmitter) Events() flow.EventsList {
	return flow.EventsList{}
}

func (NoEventEmitter) ServiceEvents() flow.EventsList {
	return flow.EventsList{}
}

func (NoEventEmitter) ConvertedServiceEvents() flow.ServiceEventList {
	return flow.ServiceEventList{}
}

func (NoEventEmitter) Reset() {
}

type eventEmitter struct {
	tracer tracing.TracerSpan
	meter  Meter

	chain   flow.Chain
	txID    flow.Identifier
	txIndex uint32
	payer   flow.Address

	EventEmitterParams
	eventCollection *EventCollection
}

// NewEventEmitter constructs a new eventEmitter
func NewEventEmitter(
	tracer tracing.TracerSpan,
	meter Meter,
	chain flow.Chain,
	txInfo TransactionInfoParams,
	params EventEmitterParams,
) EventEmitter {
	emitter := &eventEmitter{
		tracer:             tracer,
		meter:              meter,
		chain:              chain,
		txID:               txInfo.TxId,
		txIndex:            txInfo.TxIndex,
		payer:              txInfo.TxBody.Payer,
		EventEmitterParams: params,
	}

	emitter.Reset()
	return emitter
}

func (emitter *eventEmitter) Reset() {
	// TODO: for now we are not resetting meter here because we don't check meter
	//		 metrics after the first metering failure and when limit is disabled.
	emitter.eventCollection = NewEventCollection(emitter.meter)
}

func (emitter *eventEmitter) EventCollection() *EventCollection {
	return emitter.eventCollection
}

func (emitter *eventEmitter) EmitEvent(event cadence.Event) error {
	defer emitter.tracer.StartExtensiveTracingChildSpan(
		trace.FVMEnvEmitEvent).End()

	err := emitter.meter.MeterComputation(ComputationKindEmitEvent, 1)
	if err != nil {
		return fmt.Errorf("emit event failed: %w", err)
	}

	payload, err := emitter.EventEncoder.Encode(event)
	if err != nil {
		return errors.NewEventEncodingError(err)
	}

	payloadSize := uint64(len(payload))

	flowEvent := flow.Event{
		Type:             flow.EventType(event.EventType.ID()),
		TransactionID:    emitter.txID,
		TransactionIndex: emitter.txIndex,
		EventIndex:       emitter.eventCollection.TotalEventCounter(),
		Payload:          payload,
	}

	// TODO: to set limit to maximum when it is service account and get rid of this flag
	isServiceAccount := emitter.payer == emitter.chain.ServiceAddress()

	if emitter.ServiceEventCollectionEnabled {
		ok, err := IsServiceEvent(event, emitter.chain.ChainID())
		if err != nil {
			return fmt.Errorf("unable to check service event: %w", err)
		}
		if ok {
			eventEmitError := emitter.eventCollection.AppendServiceEvent(
				emitter.chain,
				flowEvent,
				payloadSize)

			// skip limit if payer is service account
			if !isServiceAccount && eventEmitError != nil {
				return eventEmitError
			}
		}
		// We don't return and append the service event into event collection
		// as well.
	}

	eventEmitError := emitter.eventCollection.AppendEvent(flowEvent, payloadSize)
	// skip limit if payer is service account
	if !isServiceAccount {
		return eventEmitError
	}

	return nil
}

func (emitter *eventEmitter) Events() flow.EventsList {
	return emitter.eventCollection.events
}

func (emitter *eventEmitter) ServiceEvents() flow.EventsList {
	return emitter.eventCollection.serviceEvents
}

func (emitter *eventEmitter) ConvertedServiceEvents() flow.ServiceEventList {
	return emitter.eventCollection.convertedServiceEvents
}

type EventCollection struct {
	events                 flow.EventsList
	serviceEvents          flow.EventsList
	convertedServiceEvents flow.ServiceEventList
	eventCounter           uint32
	meter                  Meter
}

func NewEventCollection(meter Meter) *EventCollection {
	return &EventCollection{
		events:                 make(flow.EventsList, 0, 10),
		serviceEvents:          make(flow.EventsList, 0, 10),
		convertedServiceEvents: make(flow.ServiceEventList, 0, 10),
		eventCounter:           uint32(0),
		meter:                  meter,
	}
}

func (collection *EventCollection) Events() flow.EventsList {
	return collection.events
}

func (collection *EventCollection) AppendEvent(event flow.Event, size uint64) error {
	collection.events = append(collection.events, event)
	collection.eventCounter++
	return collection.meter.MeterEmittedEvent(size)
}

func (collection *EventCollection) ServiceEvents() flow.EventsList {
	return collection.serviceEvents
}

func (collection *EventCollection) ConvertedServiceEvents() flow.ServiceEventList {
	return collection.convertedServiceEvents
}

func (collection *EventCollection) AppendServiceEvent(
	chain flow.Chain,
	event flow.Event,
	size uint64,
) error {
	convertedEvent, err := convert.ServiceEvent(chain.ChainID(), event)
	if err != nil {
		return fmt.Errorf("could not convert service event: %w", err)
	}

	collection.serviceEvents = append(collection.serviceEvents, event)
	collection.convertedServiceEvents = append(
		collection.convertedServiceEvents,
		*convertedEvent)
	collection.eventCounter++
	return collection.meter.MeterEmittedEvent(size)
}

func (collection *EventCollection) TotalByteSize() uint64 {
	return collection.meter.TotalEmittedEventBytes()
}

func (collection *EventCollection) TotalEventCounter() uint32 {
	return collection.eventCounter
}

// IsServiceEvent determines whether or not an emitted Cadence event is
// considered a service event for the given chain.
func IsServiceEvent(event cadence.Event, chain flow.ChainID) (bool, error) {

	// retrieve the service event information for this chain
	events, err := systemcontracts.ServiceEventsForChain(chain)
	if err != nil {
		return false, fmt.Errorf(
			"unknown system contracts for chain (%s): %w",
			chain.String(),
			err)
	}

	eventType := flow.EventType(event.EventType.ID())
	for _, serviceEvent := range events.All() {
		if serviceEvent.EventType() == eventType {
			return true, nil
		}
	}

	return false, nil
}
