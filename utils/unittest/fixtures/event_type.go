package fixtures

import (
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/stdlib"

	"github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/flow"
)

const (
	protocolEventName = "flow"
)

var (
	sampleContractNames = []string{"TestContract", "MyContract", "EventContract", "SampleContract", "DemoContract"}
	sampleEventNames    = []string{"TestEvent", "MyEvent", "SampleEvent", "DemoEvent", "Created", "Updated", "Deleted"}
)

// EventTypeGenerator generates event types with consistent randomness.
type EventTypeGenerator struct {
	randomGen  *RandomGenerator
	addressGen *AddressGenerator
}

func NewEventTypeGenerator(
	randomGen *RandomGenerator,
	addressGen *AddressGenerator,
) *EventTypeGenerator {
	return &EventTypeGenerator{
		randomGen:  randomGen,
		addressGen: addressGen,
	}
}

// eventTypeConfig holds the configuration for event type generation.
type eventTypeConfig struct {
	address      flow.Address
	contractName string
	eventName    string
}

// WithAddress is an option that sets the address for the event type.
func (g *EventTypeGenerator) WithAddress(address flow.Address) func(*eventTypeConfig) {
	return func(config *eventTypeConfig) {
		config.address = address
	}
}

// WithContractName is an option that sets the contract name for the event type.
func (g *EventTypeGenerator) WithContractName(contractName string) func(*eventTypeConfig) {
	return func(config *eventTypeConfig) {
		config.contractName = contractName
	}
}

// WithEventName is an option that sets the event name for the event type.
func (g *EventTypeGenerator) WithEventName(eventName string) func(*eventTypeConfig) {
	return func(config *eventTypeConfig) {
		config.eventName = eventName
	}
}

// Fixture generates a [flow.EventType] with random data based on the provided options.
func (g *EventTypeGenerator) Fixture(opts ...func(*eventTypeConfig)) flow.EventType {
	config := &eventTypeConfig{
		address:      g.addressGen.Fixture(),
		contractName: g.generateContractName(),
		eventName:    g.generateEventName(),
	}

	for _, opt := range opts {
		opt(config)
	}

	if config.contractName == protocolEventName {
		return flow.EventType(fmt.Sprintf("%s.%s", protocolEventName, config.eventName))
	}

	return flow.EventType(fmt.Sprintf("A.%s.%s.%s", config.address, config.contractName, config.eventName))
}

// List generates a list of [flow.EventType].
func (g *EventTypeGenerator) List(n int, opts ...func(*eventTypeConfig)) []flow.EventType {
	types := make([]flow.EventType, n)
	for i := range n {
		types[i] = g.Fixture(opts...)
	}
	return types
}

// generateContractName generates a random contract name.
func (g *EventTypeGenerator) generateContractName() string {
	return RandomElement(g.randomGen, sampleContractNames)
}

// generateEventName generates a random event name.
func (g *EventTypeGenerator) generateEventName() string {
	return RandomElement(g.randomGen, sampleEventNames)
}

// ToCadenceEventType converts a flow.EventType to a cadence.EventType.
func ToCadenceEventType(eventType flow.EventType) *cadence.EventType {
	parsed, err := events.ParseEvent(eventType)
	NoError(err)

	// TODO: add support for actual protocol event fields
	if parsed.Type == events.ProtocolEventType {
		return cadence.NewEventType(
			stdlib.FlowLocation{},
			parsed.Name,
			[]cadence.Field{
				{
					Identifier: "value",
					Type:       cadence.IntType,
				},
				{
					Identifier: "message",
					Type:       cadence.StringType,
				},
			},
			nil,
		)
	}

	address, err := common.BytesToAddress(flow.HexToAddress(parsed.Address).Bytes())
	NoError(err)

	return cadence.NewEventType(
		common.NewAddressLocation(nil, address, parsed.ContractName),
		fmt.Sprintf("%s.%s", parsed.ContractName, parsed.Name),
		[]cadence.Field{
			{
				Identifier: "value",
				Type:       cadence.IntType,
			},
			{
				Identifier: "message",
				Type:       cadence.StringType,
			},
		},
		nil,
	)
}
