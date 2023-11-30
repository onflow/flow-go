package environment_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/stdlib"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
)

func Test_IsServiceEvent(t *testing.T) {

	chain := flow.Emulator
	events, err := systemcontracts.ServiceEventsForChain(chain)
	require.NoError(t, err)

	t.Run("correct", func(t *testing.T) {
		for _, event := range events.All() {
			event := cadence.Event{
				EventType: &cadence.EventType{
					Location: common.AddressLocation{
						Address: common.MustBytesToAddress(
							event.Address.Bytes()),
					},
					QualifiedIdentifier: event.QualifiedIdentifier(),
				},
			}

			isServiceEvent, err := environment.IsServiceEvent(flow.EventType(event.Type().ID()), chain)
			require.NoError(t, err)
			assert.True(t, isServiceEvent)
		}
	})

	t.Run("wrong chain", func(t *testing.T) {
		event := cadence.Event{
			EventType: &cadence.EventType{
				Location: common.AddressLocation{
					Address: common.MustBytesToAddress(
						flow.Testnet.Chain().ServiceAddress().Bytes()),
				},
				QualifiedIdentifier: events.EpochCommit.QualifiedIdentifier(),
			},
		}

		isServiceEvent, err := environment.IsServiceEvent(flow.EventType(event.Type().ID()), chain)
		require.NoError(t, err)
		assert.False(t, isServiceEvent)
	})

	t.Run("wrong type", func(t *testing.T) {
		event := cadence.Event{
			EventType: &cadence.EventType{
				Location: common.AddressLocation{
					Address: common.MustBytesToAddress(
						chain.Chain().ServiceAddress().Bytes()),
				},
				QualifiedIdentifier: "SomeContract.SomeEvent",
			},
		}

		isServiceEvent, err := environment.IsServiceEvent(flow.EventType(event.Type().ID()), chain)
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
	txnState := state.NewTransactionState(
		nil,
		state.DefaultParameters().WithMeterParameters(
			meter.DefaultParameters().WithEventEmitByteLimit(eventEmitLimit),
		))

	return environment.NewEventEmitter(
		tracing.NewTracerSpan(),
		environment.NewMeter(txnState),
		chain.Chain(),
		environment.TransactionInfoParams{
			TxId:    flow.ZeroID,
			TxIndex: 0,
			TxBody: &flow.TransactionBody{
				Payer: address,
			},
		},
		environment.EventEmitterParams{
			ServiceEventCollectionEnabled: false,
			EventCollectionByteSizeLimit:  eventEmitLimit,
			EventEncoder:                  environment.NewCadenceEventEncoder(),
		},
	)
}

func getCadenceEventPayloadByteSize(event cadence.Event) uint64 {
	payload, err := ccf.Encode(event)
	if err != nil {
		panic(err)
	}

	return uint64(len(payload))
}
