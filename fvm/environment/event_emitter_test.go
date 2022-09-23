package environment_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/stdlib"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/fvm/utils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

func Test_IsServiceEvent(t *testing.T) {

	chain := flow.Emulator
	events, err := systemcontracts.ServiceEventsForChain(chain)
	require.NoError(t, err)

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

func Test_EmitEvent_Limit(t *testing.T) {
	t.Run("emit event - service account - within limit", func(t *testing.T) {
		cadenceEvent1 := cadence.Event{
			EventType: &cadence.EventType{
				Location:            stdlib.FlowLocation{},
				QualifiedIdentifier: "test",
			},
		}

		event1Size := getCadenceEventPayloadByteSize(cadenceEvent1)
		eventEmitter := createTestEventEmitterWithLimit(
			flow.Emulator,
			flow.Emulator.Chain().ServiceAddress(),
			event1Size+1)

		err := eventEmitter.EmitEvent(cadenceEvent1)
		require.NoError(t, err)
	})

	t.Run("emit event - service account - exceeding limit", func(t *testing.T) {
		cadenceEvent1 := cadence.Event{
			EventType: &cadence.EventType{
				Location:            stdlib.FlowLocation{},
				QualifiedIdentifier: "test",
			},
		}

		event1Size := getCadenceEventPayloadByteSize(cadenceEvent1)
		eventEmitter := createTestEventEmitterWithLimit(
			flow.Emulator,
			flow.Emulator.Chain().ServiceAddress(),
			event1Size-1)

		err := eventEmitter.EmitEvent(cadenceEvent1)
		require.NoError(t, err) // service count doesn't have limit
	})

	t.Run("emit event - non service account - within limit", func(t *testing.T) {
		cadenceEvent1 := cadence.Event{
			EventType: &cadence.EventType{
				Location:            stdlib.FlowLocation{},
				QualifiedIdentifier: "test",
			},
		}

		event1Size := getCadenceEventPayloadByteSize(cadenceEvent1)
		eventEmitter := createTestEventEmitterWithLimit(
			flow.Emulator,
			flow.Emulator.Chain().NewAddressGenerator().CurrentAddress(),
			event1Size+1)

		err := eventEmitter.EmitEvent(cadenceEvent1)
		require.NoError(t, err)
	})

	t.Run("emit event - non service account - exceeding limit", func(t *testing.T) {
		cadenceEvent1 := cadence.Event{
			EventType: &cadence.EventType{
				Location:            stdlib.FlowLocation{},
				QualifiedIdentifier: "test",
			},
		}

		event1Size := getCadenceEventPayloadByteSize(cadenceEvent1)
		eventEmitter := createTestEventEmitterWithLimit(
			flow.Emulator,
			flow.Emulator.Chain().NewAddressGenerator().CurrentAddress(),
			event1Size-1)

		err := eventEmitter.EmitEvent(cadenceEvent1)
		require.Error(t, err)
	})

}

func createTestEventEmitterWithLimit(chain flow.ChainID, address flow.Address, eventEmitLimit uint64) environment.EventEmitter {
	view := utils.NewSimpleView()
	stTxn := state.NewStateTransaction(
		view,
		state.DefaultParameters().WithMeterParameters(
			meter.DefaultParameters().WithEventEmitByteLimit(eventEmitLimit),
		))

	return environment.NewEventEmitter(
		environment.NewTracer(trace.NewNoopTracer(), trace.NoopSpan, false),
		environment.NewMeter(stTxn),
		chain.Chain(),
		flow.ZeroID,
		0,
		address,
		environment.EventEmitterParams{
			ServiceEventCollectionEnabled: false,
			EventCollectionByteSizeLimit:  eventEmitLimit,
		},
	)
}

func getCadenceEventPayloadByteSize(event cadence.Event) uint64 {
	payload, err := jsoncdc.Encode(event)
	if err != nil {
		panic(err)
	}

	return uint64(len(payload))
}
