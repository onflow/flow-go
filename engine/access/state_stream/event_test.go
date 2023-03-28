package state_stream_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/model/flow"
)

func TestParseEvent(t *testing.T) {
	tests := []struct {
		name      string
		eventType flow.EventType
		expected  state_stream.ParsedEvent
	}{
		{
			name:      "flow event",
			eventType: "flow.AccountCreated",
			expected: state_stream.ParsedEvent{
				Type:      state_stream.ProtocolEventType,
				EventType: "flow.AccountCreated",
				Contract:  "flow",
				Name:      "AccountCreated",
			},
		},
		{
			name:      "account event",
			eventType: "A.0000000000000001.Contract1.EventA",
			expected: state_stream.ParsedEvent{
				Type:      state_stream.AccountEventType,
				EventType: "A.0000000000000001.Contract1.EventA",
				Address:   "0000000000000001",
				Contract:  "Contract1",
				Name:      "EventA",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event, err := state_stream.ParseEvent(test.eventType)
			require.NoError(t, err)

			assert.Equal(t, test.expected.Type, event.Type, "event Type does not match")
			assert.Equal(t, test.expected.EventType, event.EventType, "event EventType does not match")
			assert.Equal(t, test.expected.Address, event.Address, "event Address does not match")
			assert.Equal(t, test.expected.Contract, event.Contract, "event Contract does not match")
			assert.Equal(t, test.expected.Name, event.Name, "event Name does not match")
		})
	}
}

func TestParseEvent_Invalid(t *testing.T) {
	eventTypes := []flow.EventType{
		"invalid",                          // not enough parts
		"invalid.event",                    // invalid first part
		"B.0000000000000001.invalid.event", // invalid first part
		"flow",                             // incorrect number of parts for protocol event
		"flow.invalid.event",               // incorrect number of parts for protocol event
		"A.0000000000000001.invalid",       // incorrect number of parts for account event
		"A.0000000000000001.invalid.a.b",   // incorrect number of parts for account event

	}

	for _, eventType := range eventTypes {
		_, err := state_stream.ParseEvent(eventType)
		assert.Error(t, err, "expected error for event type: %s", eventType)
	}
}
