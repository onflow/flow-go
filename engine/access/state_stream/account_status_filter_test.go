package state_stream_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/generator"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/model/flow"
)

// TestAccountStatusFilterContructor tests the constructor of the AccountStatusFilter with different scenarios.
func TestAccountStatusFilterContructor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		eventTypes       []string
		accountAddresses []string
		err              bool
	}{
		{
			name: "no filters, no addresses",
		},
		{
			name:       "valid filters, no addresses",
			eventTypes: []string{state_stream.CoreEventAccountCreated, state_stream.CoreEventAccountContractAdded, state_stream.CoreEventInboxValueClaimed},
		},
		{
			name:       "invalid filters, no addresses",
			eventTypes: []string{state_stream.CoreEventAccountCreated, "A.0000000000000001.Contract1.EventA"},
			err:        true,
		},
		{
			name:             "no filters, valid addresses",
			accountAddresses: []string{"0000000000000001", "0000000000000002", "0000000000000003"},
		},
		{
			name:             "valid filters, valid addresses",
			eventTypes:       []string{state_stream.CoreEventAccountCreated, state_stream.CoreEventAccountContractAdded, state_stream.CoreEventInboxValueClaimed},
			accountAddresses: []string{"0000000000000001", "0000000000000002", "0000000000000003"},
		},
		{
			name:             "invalid filters, valid addresses",
			eventTypes:       []string{state_stream.CoreEventAccountCreated, "A.0000000000000001.Contract1.EventA"},
			accountAddresses: []string{"0000000000000001", "0000000000000002", "0000000000000003"},
			err:              true,
		},
		{
			name:             "valid filters, invalid addresses",
			eventTypes:       []string{state_stream.CoreEventAccountCreated, state_stream.CoreEventAccountContractAdded, state_stream.CoreEventInboxValueClaimed},
			accountAddresses: []string{"invalid"},
			err:              true,
		},
	}

	chain := flow.MonotonicEmulator.Chain()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			filter, err := state_stream.NewAccountStatusFilter(state_stream.DefaultEventFilterConfig, chain, test.eventTypes, test.accountAddresses)

			if test.err {
				assert.Error(t, err)
				assert.Equal(t, filter, state_stream.AccountStatusFilter{})
			} else {
				assert.NoError(t, err)

				if len(test.eventTypes) == 0 {
					assert.Equal(t, len(filter.EventTypes), len(state_stream.DefaultCoreEvents))
				} else {
					assert.Equal(t, len(filter.EventTypes), len(test.eventTypes))
				}

				for key := range filter.EventTypes {
					switch key {
					case state_stream.CoreEventAccountCreated,
						state_stream.CoreEventAccountContractAdded:
						assert.Equal(t, len(filter.EventFieldFilters[key]["address"]), len(test.accountAddresses))
					case state_stream.CoreEventInboxValueClaimed:
						assert.Equal(t, len(filter.EventFieldFilters[key]["provider"]), len(test.accountAddresses))
					}
				}
			}
		})
	}
}

// TestAccountStatusFilterFiltering tests the filtering mechanism of the AccountStatusFilter.
// It verifies that the filter correctly filters the events based on the provided event types and account addresses.
func TestAccountStatusFilterFiltering(t *testing.T) {
	chain := flow.MonotonicEmulator.Chain()

	filterEventTypes := []string{state_stream.CoreEventAccountCreated, state_stream.CoreEventAccountContractAdded}

	addressGenerator := chain.NewAddressGenerator()
	addressAccountCreate, err := addressGenerator.NextAddress()
	require.NoError(t, err)

	accountContractAddedAddress, err := addressGenerator.NextAddress()
	require.NoError(t, err)

	filter, err := state_stream.NewAccountStatusFilter(
		state_stream.DefaultEventFilterConfig,
		chain,
		filterEventTypes,
		[]string{addressAccountCreate.HexWithPrefix(), accountContractAddedAddress.HexWithPrefix()},
	)
	require.NoError(t, err)

	accountCreateEvent := generator.GenerateAccountCreateEvent(t, addressAccountCreate)
	accountContractAdded := generator.GenerateAccountContractEvent(t, "AccountContractAdded", accountContractAddedAddress)

	events := flow.EventsList{
		unittest.EventFixture("A.0000000000000001.Contract1.EventA", 0, 0, unittest.IdentifierFixture(), 0),
		accountCreateEvent,
		unittest.EventFixture("A.0000000000000001.Contract2.EventA", 0, 0, unittest.IdentifierFixture(), 0),
		accountContractAdded,
	}

	matched := filter.Filter(events)
	matchedByAddress := filter.GroupCoreEventsByAccountAddress(matched, unittest.Logger())

	assert.Len(t, matched, 2)

	assert.Equal(t, events[1], matched[0])
	matchAccCreated, ok := matchedByAddress[addressAccountCreate.HexWithPrefix()]
	require.True(t, ok)
	assert.Equal(t, flow.EventsList{accountCreateEvent}, matchAccCreated)

	assert.Equal(t, events[3], matched[1])
	matchContractAdded, ok := matchedByAddress[accountContractAddedAddress.HexWithPrefix()]
	require.True(t, ok)
	assert.Equal(t, flow.EventsList{accountContractAdded}, matchContractAdded)
}
