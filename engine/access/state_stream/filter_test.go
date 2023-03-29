package state_stream_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/model/flow"
)

var eventTypes = map[flow.EventType]bool{
	"flow.AccountCreated":                 true,
	"flow.AccountKeyAdded":                true,
	"A.0000000000000001.Contract1.EventA": true,
	"A.0000000000000001.Contract1.EventB": true,
	"A.0000000000000001.Contract2.EventA": true,
	"A.0000000000000001.Contract3.EventA": true,
	"A.0000000000000002.Contract1.EventA": true,
	"A.0000000000000002.Contract4.EventC": true,
	"A.0000000000000003.Contract5.EventA": true,
	"A.0000000000000003.Contract5.EventD": true,
	"A.0000000000000004.Contract6.EventE": true,
}

func TestContructor(t *testing.T) {
	tests := []struct {
		name       string
		eventTypes []string
		addresses  []string
		contracts  []string
		eventNames []string
		err        bool
	}{
		{
			name: "no filters",
		},
		{
			name:       "valid filters",
			eventTypes: []string{"flow.AccountCreated", "A.0000000000000001.Contract1.EventA"},
			addresses:  []string{"0000000000000001", "0000000000000002"},
			contracts:  []string{"A.0000000000000001.Contract1", "A.0000000000000001.Contract2"},
			eventNames: []string{"EventA", "EventB"},
		},
		{
			name:       "invalid event type",
			eventTypes: []string{"invalid"},
			err:        true,
		},
		{
			name:      "invalid address",
			addresses: []string{"invalid"},
			err:       true,
		},
		{
			name:      "invalid contract",
			contracts: []string{"invalid.contract"},
			err:       true,
		},
		{
			name:       "invalid event name",
			eventNames: []string{"invalid.event"},
			err:        true,
		},
	}

	chain := flow.MonotonicEmulator.Chain()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			filter, err := state_stream.NewEventFilter(chain, test.eventTypes, test.addresses, test.contracts, test.eventNames)
			if test.err {
				assert.Error(t, err)
				assert.Equal(t, filter, state_stream.EventFilter{})
			} else {
				assert.NoError(t, err)
				assert.Len(t, filter.EventTypes, len(test.eventTypes))
				assert.Len(t, filter.Addresses, len(test.addresses))
				assert.Len(t, filter.Contracts, len(test.contracts))
				assert.Len(t, filter.EventNames, len(test.eventNames))
			}
		})
	}
}

func TestMatch(t *testing.T) {
	tests := []struct {
		name       string
		eventTypes []string
		addresses  []string
		contracts  []string
		eventNames []string
		matches    map[flow.EventType]bool
	}{
		{
			name:    "no filters",
			matches: eventTypes,
		},
		{
			name:       "eventtype filter",
			eventTypes: []string{"flow.AccountCreated", "A.0000000000000001.Contract1.EventA"},
			matches: map[flow.EventType]bool{
				"flow.AccountCreated":                 true,
				"A.0000000000000001.Contract1.EventA": true,
			},
		},
		{
			name:      "address filter",
			addresses: []string{"0000000000000001", "0000000000000002"},
			matches: map[flow.EventType]bool{
				"A.0000000000000001.Contract1.EventA": true,
				"A.0000000000000001.Contract1.EventB": true,
				"A.0000000000000001.Contract2.EventA": true,
				"A.0000000000000001.Contract3.EventA": true,
				"A.0000000000000002.Contract1.EventA": true,
				"A.0000000000000002.Contract4.EventC": true,
			},
		},
		{
			name:      "contract filter",
			contracts: []string{"A.0000000000000001.Contract1", "A.0000000000000002.Contract4"},
			matches: map[flow.EventType]bool{
				"A.0000000000000001.Contract1.EventA": true,
				"A.0000000000000001.Contract1.EventB": true,
				"A.0000000000000002.Contract4.EventC": true,
			},
		},
		{
			name:       "eventname filter",
			eventNames: []string{"EventA", "EventC"},
			matches: map[flow.EventType]bool{
				"A.0000000000000001.Contract1.EventA": true,
				"A.0000000000000001.Contract2.EventA": true,
				"A.0000000000000001.Contract3.EventA": true,
				"A.0000000000000002.Contract1.EventA": true,
				"A.0000000000000002.Contract4.EventC": true,
				"A.0000000000000003.Contract5.EventA": true,
			},
		},
		{
			name:       "multiple filters",
			eventTypes: []string{"A.0000000000000001.Contract1.EventA"},
			addresses:  []string{"0000000000000002"},
			contracts:  []string{"flow", "A.0000000000000001.Contract1", "A.0000000000000001.Contract2"},
			eventNames: []string{"EventD"},
			matches: map[flow.EventType]bool{
				"flow.AccountCreated":                 true,
				"flow.AccountKeyAdded":                true,
				"A.0000000000000001.Contract1.EventA": true,
				"A.0000000000000001.Contract1.EventB": true,
				"A.0000000000000001.Contract2.EventA": true,
				"A.0000000000000002.Contract1.EventA": true,
				"A.0000000000000002.Contract4.EventC": true,
				"A.0000000000000003.Contract5.EventD": true,
			},
		},
	}

	events := make([]flow.Event, 0, len(eventTypes))
	for eventType := range eventTypes {
		events = append(events, flow.Event{Type: flow.EventType(eventType)})
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, address := range test.addresses {
				t.Log(flow.HexToAddress(address))
			}
			filter, err := state_stream.NewEventFilter(flow.MonotonicEmulator.Chain(), test.eventTypes, test.addresses, test.contracts, test.eventNames)
			assert.NoError(t, err)
			for _, event := range events {
				assert.Equal(t, test.matches[event.Type], filter.Match(event), "event type: %s", event.Type)
			}
		})
	}
}
