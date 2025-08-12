package fixtures

import (
	"fmt"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/stdlib"
	"github.com/stretchr/testify/require"

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

// eventTypeConfig holds the configuration for event type generation.
type eventTypeConfig struct {
	address      flow.Address
	contractName string
	eventName    string
}

// WithAddress returns an option to set the address for the event type.
func (g *EventTypeGenerator) WithAddress(address flow.Address) func(*eventTypeConfig) {
	return func(config *eventTypeConfig) {
		config.address = address
	}
}

// WithContractName returns an option to set the contract name for the event type.
func (g *EventTypeGenerator) WithContractName(contractName string) func(*eventTypeConfig) {
	return func(config *eventTypeConfig) {
		config.contractName = contractName
	}
}

// WithEventName returns an option to set the event name for the event type.
func (g *EventTypeGenerator) WithEventName(eventName string) func(*eventTypeConfig) {
	return func(config *eventTypeConfig) {
		config.eventName = eventName
	}
}

// Fixture generates an event type with optional configuration.
func (g *EventTypeGenerator) Fixture(t testing.TB, opts ...func(*eventTypeConfig)) flow.EventType {
	config := &eventTypeConfig{
		address:      g.addressGen.Fixture(t),
		contractName: g.generateContractName(t),
		eventName:    g.generateEventName(t),
	}

	for _, opt := range opts {
		opt(config)
	}

	if config.contractName == protocolEventName {
		return flow.EventType(fmt.Sprintf("%s.%s", protocolEventName, config.eventName))
	}

	return flow.EventType(fmt.Sprintf("A.%s.%s.%s", config.address, config.contractName, config.eventName))
}

// List generates a list of event types.
func (g *EventTypeGenerator) List(t testing.TB, n int, opts ...func(*eventTypeConfig)) []flow.EventType {
	types := make([]flow.EventType, n)
	for i := range n {
		types[i] = g.Fixture(t, opts...)
	}
	return types
}

// generateContractName generates a random contract name.
func (g *EventTypeGenerator) generateContractName(t testing.TB) string {
	return sampleContractNames[g.randomGen.Intn(len(sampleContractNames))]
}

// generateEventName generates a random event name.
func (g *EventTypeGenerator) generateEventName(t testing.TB) string {
	return sampleEventNames[g.randomGen.Intn(len(sampleEventNames))]
}

// ToCadenceEventType converts a flow.EventType to a cadence.EventType.
func ToCadenceEventType(t testing.TB, eventType flow.EventType) *cadence.EventType {
	parsed, err := events.ParseEvent(eventType)
	require.NoError(t, err)

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
	require.NoError(t, err)

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
