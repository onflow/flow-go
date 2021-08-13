package handler_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/handler"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

func Test_IsServiceEvent(t *testing.T) {

	chain := flow.Emulator
	events, err := systemcontracts.ServiceEventsForChain(chain)
	require.NoError(t, err)

	t.Run("correct", func(t *testing.T) {
		for _, event := range events.All() {
			isServiceEvent, err := handler.IsServiceEvent(cadence.Event{
				EventType: &cadence.EventType{
					Location: common.AddressLocation{
						Address: common.BytesToAddress(event.Address.Bytes()),
					},
					QualifiedIdentifier: event.QualifiedIdentifier(),
				},
			}, chain)
			require.NoError(t, err)
			assert.True(t, isServiceEvent)
		}
	})

	t.Run("wrong chain", func(t *testing.T) {
		isServiceEvent, err := handler.IsServiceEvent(cadence.Event{
			EventType: &cadence.EventType{
				Location: common.AddressLocation{
					Address: common.BytesToAddress(flow.Testnet.Chain().ServiceAddress().Bytes()),
				},
				QualifiedIdentifier: events.EpochCommit.QualifiedIdentifier(),
			},
		}, chain)
		require.NoError(t, err)
		assert.False(t, isServiceEvent)
	})

	t.Run("wrong type", func(t *testing.T) {
		isServiceEvent, err := handler.IsServiceEvent(cadence.Event{
			EventType: &cadence.EventType{
				Location: common.AddressLocation{
					Address: common.BytesToAddress(chain.Chain().ServiceAddress().Bytes()),
				},
				QualifiedIdentifier: "SomeContract.SomeEvent",
			},
		}, chain)
		require.NoError(t, err)
		assert.False(t, isServiceEvent)
	})
}
