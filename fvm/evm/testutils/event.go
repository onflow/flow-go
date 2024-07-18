package testutils

import (
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/onflow/cadence/runtime/common"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"

	"github.com/onflow/flow-go/fvm/evm/events"
	"github.com/onflow/flow-go/model/flow"
)

func flowToCadenceEvent(t testing.TB, event flow.Event) cadence.Event {
	ev, err := ccf.Decode(nil, event.Payload)
	require.NoError(t, err)
	cadenceEvent, ok := ev.(cadence.Event)
	require.True(t, ok)
	return cadenceEvent
}

func TxEventToPayload(t testing.TB, event flow.Event, evmContract flow.Address) *events.TransactionEventPayload {
	assert.Equal(
		t,
		common.NewAddressLocation(
			nil,
			common.Address(evmContract),
			string(events.EventTypeTransactionExecuted),
		).ID(),
		string(event.Type),
	)
	cadenceEvent := flowToCadenceEvent(t, event)
	txEventPayload, err := events.DecodeTransactionEventPayload(cadenceEvent)
	require.NoError(t, err)
	return txEventPayload
}

func BlockEventToPayload(t testing.TB, event flow.Event, evmContract flow.Address) *events.BlockEventPayload {
	assert.Equal(
		t,
		common.NewAddressLocation(
			nil,
			common.Address(evmContract),
			string(events.EventTypeBlockExecuted),
		).ID(),
		string(event.Type),
	)

	cadenceEvent := flowToCadenceEvent(t, event)
	blockEventPayload, err := events.DecodeBlockEventPayload(cadenceEvent)
	require.NoError(t, err)
	return blockEventPayload
}
