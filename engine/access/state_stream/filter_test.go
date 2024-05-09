package state_stream_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
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
	t.Parallel()

	tests := []struct {
		name       string
		eventTypes []string
		addresses  []string
		contracts  []string
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
	}

	chain := flow.MonotonicEmulator.Chain()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			filter, err := state_stream.NewEventFilter(state_stream.DefaultEventFilterConfig, chain, test.eventTypes, test.addresses, test.contracts)
			if test.err {
				assert.Error(t, err)
				assert.Equal(t, filter, state_stream.EventFilter{})
			} else {
				assert.NoError(t, err)
				assert.Len(t, filter.EventTypes, len(test.eventTypes))
				assert.Len(t, filter.Addresses, len(test.addresses))
				assert.Len(t, filter.Contracts, len(test.contracts))
			}
		})
	}
}

func TestFilter(t *testing.T) {
	t.Parallel()

	chain := flow.MonotonicEmulator.Chain()

	filter, err := state_stream.NewEventFilter(state_stream.DefaultEventFilterConfig, chain, []string{"flow.AccountCreated", "A.0000000000000001.Contract1.EventA"}, nil, nil)
	assert.NoError(t, err)

	events := flow.EventsList{
		unittest.EventFixture("A.0000000000000001.Contract1.EventA", 0, 0, unittest.IdentifierFixture(), 0),
		unittest.EventFixture("A.0000000000000001.Contract2.EventA", 0, 0, unittest.IdentifierFixture(), 0),
		unittest.EventFixture("flow.AccountCreated", 0, 0, unittest.IdentifierFixture(), 0),
	}

	matched := filter.Filter(events)

	assert.Len(t, matched, 2)
	assert.Equal(t, events[0], matched[0])
	assert.Equal(t, events[2], matched[1])
}

func TestMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		eventTypes []string
		addresses  []string
		contracts  []string
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
			name:       "multiple filters",
			eventTypes: []string{"A.0000000000000001.Contract1.EventA"},
			addresses:  []string{"0000000000000002"},
			contracts:  []string{"flow", "A.0000000000000001.Contract1", "A.0000000000000001.Contract2"},
			matches: map[flow.EventType]bool{
				"flow.AccountCreated":                 true,
				"flow.AccountKeyAdded":                true,
				"A.0000000000000001.Contract1.EventA": true,
				"A.0000000000000001.Contract1.EventB": true,
				"A.0000000000000001.Contract2.EventA": true,
				"A.0000000000000002.Contract1.EventA": true,
				"A.0000000000000002.Contract4.EventC": true,
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
			filter, err := state_stream.NewEventFilter(
				state_stream.DefaultEventFilterConfig,
				flow.MonotonicEmulator.Chain(),
				test.eventTypes,
				test.addresses,
				test.contracts,
			)
			assert.NoError(t, err)
			for _, event := range events {
				assert.Equal(t, test.matches[event.Type], filter.Match(event), "event type: %s", event.Type)
			}
		})
	}
}
