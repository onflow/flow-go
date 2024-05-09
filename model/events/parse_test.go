package events_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestParseEvent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		eventType flow.EventType
		expected  events.ParsedEvent
	}{
		{
			name:      "flow event",
			eventType: "flow.AccountCreated",
			expected: events.ParsedEvent{
				Type:         events.ProtocolEventType,
				EventType:    "flow.AccountCreated",
				Contract:     "flow",
				ContractName: "flow",
				Name:         "AccountCreated",
			},
		},
		{
			name:      "account event",
			eventType: "A.0000000000000001.Contract1.EventA",
			expected: events.ParsedEvent{
				Type:         events.AccountEventType,
				EventType:    "A.0000000000000001.Contract1.EventA",
				Address:      "0000000000000001",
				Contract:     "A.0000000000000001.Contract1",
				ContractName: "Contract1",
				Name:         "EventA",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event, err := events.ParseEvent(test.eventType)
			require.NoError(t, err)

			assert.Equal(t, test.expected.Type, event.Type)
			assert.Equal(t, test.expected.EventType, event.EventType)
			assert.Equal(t, test.expected.Address, event.Address)
			assert.Equal(t, test.expected.Contract, event.Contract)
			assert.Equal(t, test.expected.Name, event.Name)
		})
	}
}

func TestParseEvent_Invalid(t *testing.T) {
	t.Parallel()

	eventTypes := []flow.EventType{
		"",                                 // not enough parts
		"invalid",                          // not enough parts
		"invalid.event",                    // invalid first part
		"B.0000000000000001.invalid.event", // invalid first part
		"flow",                             // incorrect number of parts for protocol event
		"flow.invalid.event",               // incorrect number of parts for protocol event
		"A.0000000000000001.invalid",       // incorrect number of parts for account event
		"A.0000000000000001.invalid.a.b",   // incorrect number of parts for account event

	}

	for _, eventType := range eventTypes {
		_, err := events.ParseEvent(eventType)
		assert.Error(t, err, "expected error for event type: %s", eventType)
	}
}

func TestValidateEvent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		eventType flow.EventType
		expected  events.ParsedEvent
	}{
		{
			name:      "flow event",
			eventType: "flow.AccountCreated",
			expected: events.ParsedEvent{
				Type:         events.ProtocolEventType,
				EventType:    "flow.AccountCreated",
				Contract:     "flow",
				ContractName: "flow",
				Name:         "AccountCreated",
			},
		},
		{
			name:      "account event",
			eventType: "A.0000000000000001.Contract1.EventA",
			expected: events.ParsedEvent{
				Type:         events.AccountEventType,
				EventType:    "A.0000000000000001.Contract1.EventA",
				Address:      "0000000000000001",
				Contract:     "A.0000000000000001.Contract1",
				ContractName: "Contract1",
				Name:         "EventA",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event, err := events.ValidateEvent(test.eventType, flow.MonotonicEmulator.Chain())
			require.NoError(t, err)

			assert.Equal(t, test.expected.Type, event.Type)
			assert.Equal(t, test.expected.EventType, event.EventType)
			assert.Equal(t, test.expected.Address, event.Address)
			assert.Equal(t, test.expected.Contract, event.Contract)
			assert.Equal(t, test.expected.Name, event.Name)
		})
	}
}

func TestValidateEvent_Invalid(t *testing.T) {
	t.Parallel()

	eventTypes := []flow.EventType{
		"",                                 // not enough parts
		"invalid",                          // not enough parts
		"invalid.event",                    // invalid first part
		"B.0000000000000001.invalid.event", // invalid first part
		"flow",                             // incorrect number of parts for protocol event
		"flow.invalid.event",               // incorrect number of parts for protocol event
		"A.0000000000000001.invalid",       // incorrect number of parts for account event
		"A.0000000000000001.invalid.a.b",   // incorrect number of parts for account event
		flow.EventType(fmt.Sprintf("A.%s.Contract1.EventA", unittest.RandomAddressFixture())), // address from wrong chain
	}

	for _, eventType := range eventTypes {
		_, err := events.ValidateEvent(eventType, flow.MonotonicEmulator.Chain())
		assert.Error(t, err, "expected error for event type: %s", eventType)
	}
}
