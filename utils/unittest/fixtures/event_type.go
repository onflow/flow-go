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

type EventTypeGenerator struct {
	randomGen  *RandomGenerator
	addressGen *AddressGenerator
}

type eventTypeConfig struct {
	address      flow.Address
	contractName string
	eventName    string
}

func (g *EventTypeGenerator) WithAddress(address flow.Address) func(*eventTypeConfig) {
	return func(config *eventTypeConfig) {
		config.address = address
	}
}

func (g *EventTypeGenerator) WithContractName(contractName string) func(*eventTypeConfig) {
	return func(config *eventTypeConfig) {
		config.contractName = contractName
	}
}

func (g *EventTypeGenerator) WithEventName(eventName string) func(*eventTypeConfig) {
	return func(config *eventTypeConfig) {
		config.eventName = eventName
	}
}

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

func (g *EventTypeGenerator) generateContractName(t testing.TB) string {
	return sampleContractNames[g.randomGen.Intn(len(sampleContractNames))]
}

func (g *EventTypeGenerator) generateEventName(t testing.TB) string {
	return sampleEventNames[g.randomGen.Intn(len(sampleEventNames))]
}

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
