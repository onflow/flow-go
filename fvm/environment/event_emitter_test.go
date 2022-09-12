package environment_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

func Test_IsServiceEvent(t *testing.T) {

	chain := flow.Emulator
	events := systemcontracts.ServiceEventsForChain(chain)

	t.Run("correct", func(t *testing.T) {
		for _, event := range events.All() {
			isServiceEvent, err := environment.IsServiceEvent(cadence.Event{
				EventType: &cadence.EventType{
					Location: common.AddressLocation{
						Address: common.Address(event.Address),
					},
					QualifiedIdentifier: event.QualifiedIdentifier(),
				},
			}, chain)
			require.NoError(t, err)
			assert.True(t, isServiceEvent)
		}
	})

	t.Run("wrong chain", func(t *testing.T) {
		isServiceEvent, err := environment.IsServiceEvent(cadence.Event{
			EventType: &cadence.EventType{
				Location: common.AddressLocation{
					Address: common.Address(flow.Testnet.Chain().ServiceAddress()),
				},
				QualifiedIdentifier: events.EpochCommit.QualifiedIdentifier(),
			},
		}, chain)
		require.NoError(t, err)
		assert.False(t, isServiceEvent)
	})

	t.Run("wrong type", func(t *testing.T) {
		isServiceEvent, err := environment.IsServiceEvent(cadence.Event{
			EventType: &cadence.EventType{
				Location: common.AddressLocation{
					Address: common.Address(chain.Chain().ServiceAddress()),
				},
				QualifiedIdentifier: "SomeContract.SomeEvent",
			},
		}, chain)
		require.NoError(t, err)
		assert.False(t, isServiceEvent)
	})
}

func Test_ScriptEventEmitter(t *testing.T) {
	// scripts use the NoEventEmitter
	emitter := environment.NoEventEmitter{}
	err := emitter.EmitEvent(cadence.Event{})
	require.NoError(t, err, "script should not error when emitting events")
}
