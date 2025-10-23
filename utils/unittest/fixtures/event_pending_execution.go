package fixtures

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/onflow/flow-core-contracts/lib/go/templates"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

// PendingExecutionEvent is the default options factory for [flow.PendingExecutionEvent] generation.
var PendingExecutionEvent pendingExecutionEventFactory

type pendingExecutionEventFactory struct{}

type PendingExecutionEventOption func(*PendingExecutionEventGenerator, *pendingExecutionEventConfig)

type pendingExecutionEventConfig struct {
	event           flow.Event
	id              uint64
	priority        uint8
	executionEffort uint64
	fees            uint64
	callbackOwner   flow.Address
}

// WithID is an option that sets the scheduled transaction's `id` field.
func (f pendingExecutionEventFactory) WithID(id uint64) PendingExecutionEventOption {
	return func(g *PendingExecutionEventGenerator, config *pendingExecutionEventConfig) {
		config.id = id
	}
}

// WithPriority is an option that sets the scheduled transaction's `priority` field.
// One of high(0), medium(1), low(2).
func (f pendingExecutionEventFactory) WithPriority(priority uint8) PendingExecutionEventOption {
	return func(g *PendingExecutionEventGenerator, config *pendingExecutionEventConfig) {
		Assert(priority <= 2, "priority must be between 0 and 2")
		config.priority = priority
	}
}

// WithExecutionEffort is an option that sets the scheduled transaction's `executionEffort` field.
func (f pendingExecutionEventFactory) WithExecutionEffort(executionEffort uint64) PendingExecutionEventOption {
	return func(g *PendingExecutionEventGenerator, config *pendingExecutionEventConfig) {
		config.executionEffort = executionEffort
	}
}

// WithFees is an option that sets the scheduled transaction's `fees` field.
// This is the uint64 representation of the cadence.UFix64 value.
func (f pendingExecutionEventFactory) WithFees(fees uint64) PendingExecutionEventOption {
	return func(g *PendingExecutionEventGenerator, config *pendingExecutionEventConfig) {
		config.fees = fees
	}
}

// WithCallbackOwner is an option that sets the scheduled transaction's `callbackOwner` field.
func (f pendingExecutionEventFactory) WithCallbackOwner(callbackOwner flow.Address) PendingExecutionEventOption {
	return func(g *PendingExecutionEventGenerator, config *pendingExecutionEventConfig) {
		config.callbackOwner = callbackOwner
	}
}

// WithTransactionID is an option that sets the transaction ID for the event.
func (f pendingExecutionEventFactory) WithTransactionID(transactionID flow.Identifier) PendingExecutionEventOption {
	return func(g *PendingExecutionEventGenerator, config *pendingExecutionEventConfig) {
		config.event.TransactionID = transactionID
	}
}

// WithTransactionIndex is an option that sets the transaction index for the event.
func (f pendingExecutionEventFactory) WithTransactionIndex(transactionIndex uint32) PendingExecutionEventOption {
	return func(g *PendingExecutionEventGenerator, config *pendingExecutionEventConfig) {
		config.event.TransactionIndex = transactionIndex
	}
}

// WithEventIndex is an option that sets the event index for the event.
func (f pendingExecutionEventFactory) WithEventIndex(eventIndex uint32) PendingExecutionEventOption {
	return func(g *PendingExecutionEventGenerator, config *pendingExecutionEventConfig) {
		config.event.EventIndex = eventIndex
	}
}

// PendingExecutionEventGenerator generates node ejection events with consistent randomness.
type PendingExecutionEventGenerator struct {
	pendingExecutionEventFactory

	random    *RandomGenerator
	addresses *AddressGenerator
	events    *EventGenerator

	chainID flow.ChainID
}

// NewPendingExecutionEventGenerator creates a new PendingExecutionEventGenerator.
func NewPendingExecutionEventGenerator(
	random *RandomGenerator,
	addresses *AddressGenerator,
	events *EventGenerator,
	chainID flow.ChainID,
) *PendingExecutionEventGenerator {
	return &PendingExecutionEventGenerator{
		random:    random,
		addresses: addresses,
		events:    events,
		chainID:   chainID,
	}
}

// Fixture generates a [flow.PendingExecutionEvent] with random data based on the provided options.
func (g *PendingExecutionEventGenerator) Fixture(opts ...PendingExecutionEventOption) flow.Event {
	env := systemcontracts.SystemContractsForChain(g.chainID).AsTemplateEnv()
	eventType := blueprints.PendingExecutionEventType(env)

	processCallbackTx, err := blueprints.ProcessCallbacksTransaction(g.chainID.Chain())
	NoError(err)

	config := &pendingExecutionEventConfig{
		id:              g.random.Uint64(),
		priority:        g.random.Uint8n(3), // high(0), medium(1), low(2)
		executionEffort: g.random.Uint64n(10_000),
		fees:            0,
		callbackOwner:   g.addresses.Fixture(),
		event: g.events.Fixture(
			Event.WithEventType(eventType),
			Event.WithTransactionID(processCallbackTx.ID()),
			Event.WithPayload([]byte{}), // prevent payload generation
		),
	}

	for _, opt := range opts {
		opt(g, config)
	}

	config.event.Payload = g.generatePayload(env, config)

	return config.event
}

// List generates a list of [flow.Event].
func (g *PendingExecutionEventGenerator) List(n int, opts ...PendingExecutionEventOption) []flow.Event {
	events := make([]flow.Event, n)
	for i := range n {
		nOpts := append(opts, PendingExecutionEvent.WithEventIndex(uint32(i)))
		events[i] = g.Fixture(nOpts...)
	}
	return events
}

// generatePayload generates the event payload for a properly formatted PendingExecution event.
func (g *PendingExecutionEventGenerator) generatePayload(
	env templates.Environment,
	config *pendingExecutionEventConfig,
) []byte {
	loc, err := common.HexToAddress(env.FlowTransactionSchedulerAddress)
	NoError(err)

	eventType := cadence.NewEventType(
		common.NewAddressLocation(nil, loc, "PendingExecution"),
		"PendingExecution",
		[]cadence.Field{
			{Identifier: "id", Type: cadence.UInt64Type},
			{Identifier: "priority", Type: cadence.UInt8Type},
			{Identifier: "executionEffort", Type: cadence.UInt64Type},
			{Identifier: "fees", Type: cadence.UFix64Type},
			{Identifier: "callbackOwner", Type: cadence.AddressType},
		},
		nil,
	)

	event := cadence.NewEvent(
		[]cadence.Value{
			cadence.NewUInt64(config.id),
			cadence.NewUInt8(config.priority),
			cadence.NewUInt64(config.executionEffort),
			cadence.UFix64(config.fees),
			cadence.NewAddress(config.callbackOwner),
		},
	).WithType(eventType)

	payload, err := ccf.Encode(event)
	NoError(err)

	return payload
}
