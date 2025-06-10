package blueprints_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	cadenceCommon "github.com/onflow/cadence/common"
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestProcessCallbacksTransaction(t *testing.T) {
	t.Parallel()

	chain := flow.Mainnet.Chain()
	tx := blueprints.ProcessCallbacksTransaction(chain)

	assert.NotNil(t, tx)
	assert.NotEmpty(t, tx.Script)
	assert.Equal(t, uint64(blueprints.SystemChunkTransactionGasLimit), tx.GasLimit)
	assert.Empty(t, tx.Arguments)
}

func TestExecuteCallbacksTransactions(t *testing.T) {
	t.Parallel()

	chain := flow.Mainnet.Chain()

	tests := []struct {
		name         string
		events       []flow.Event
		expectedTxs  int
		expectError  bool
		errorMessage string
	}{
		{
			name:        "no events",
			events:      []flow.Event{},
			expectedTxs: 0,
			expectError: false,
		},
		{
			name:        "single valid event",
			events:      []flow.Event{createValidCallbackEvent(t, 1, 100)},
			expectedTxs: 1,
			expectError: false,
		},
		{
			name: "multiple valid events",
			events: []flow.Event{
				createValidCallbackEvent(t, 1, 100),
				createValidCallbackEvent(t, 2, 200),
				createValidCallbackEvent(t, 3, 300),
			},
			expectedTxs: 3,
			expectError: false,
		},
		{
			name:         "invalid event type",
			events:       []flow.Event{createInvalidTypeEvent()},
			expectedTxs:  0,
			expectError:  true,
			errorMessage: "failed to get callback args from event",
		},
		{
			name:         "invalid event payload",
			events:       []flow.Event{createInvalidPayloadEvent()},
			expectedTxs:  0,
			expectError:  true,
			errorMessage: "failed to get callback args from event",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txs, err := blueprints.ExecuteCallbacksTransactions(chain, tt.events)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMessage)
				assert.Nil(t, txs)
				return
			}

			assert.NoError(t, err)
			assert.Len(t, txs, tt.expectedTxs)

			for i, tx := range txs {
				assert.NotNil(t, tx)
				assert.NotEmpty(t, tx.Script)
				expectedEffort := uint64(100 * (i + 1)) // Events created with efforts 100, 200, 300
				assert.Equal(t, expectedEffort, tx.GasLimit)
				assert.Len(t, tx.Arguments, 1)
				assert.NotEmpty(t, tx.Arguments[0])

				t.Logf("Transaction %d: ID arg length: %d, GasLimit: %d",
					i, len(tx.Arguments[0]), tx.GasLimit)
			}
		})
	}
}

func TestExecuteCallbackTransaction(t *testing.T) {
	t.Parallel()

	chain := flow.Mainnet.Chain()

	const id = 123
	const effort = 456
	event := createValidCallbackEvent(t, id, effort)
	txs, err := blueprints.ExecuteCallbacksTransactions(chain, []flow.Event{event})

	require.NoError(t, err)
	require.Len(t, txs, 1)

	tx := txs[0]
	assert.NotNil(t, tx)
	assert.NotEmpty(t, tx.Script)
	assert.Equal(t, uint64(effort), tx.GasLimit)
	assert.Len(t, tx.Arguments, 1)

	expectedEncodedID, err := ccf.Encode(cadence.NewUInt64(id))
	require.NoError(t, err)
	assert.Equal(t, tx.Arguments[0], expectedEncodedID)

	assert.Equal(t, tx.GasLimit, uint64(effort))
}

func createValidCallbackEvent(t *testing.T, id uint64, effort uint64) flow.Event {
	// todo use proper location
	contractAddress := flow.HexToAddress("0x0000000000000000")
	location := cadenceCommon.NewAddressLocation(nil, cadenceCommon.Address(contractAddress), "CallbackScheduler")

	eventType := cadence.NewEventType(
		location,
		"CallbackProcessed",
		[]cadence.Field{
			{Identifier: "ID", Type: cadence.UInt64Type},
			{Identifier: "executionEffort", Type: cadence.UInt64Type},
		},
		nil,
	)

	event := cadence.NewEvent(
		[]cadence.Value{
			cadence.NewUInt64(id),
			cadence.NewUInt64(effort),
		},
	).WithType(eventType)

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	return flow.Event{
		Type:             flow.EventType("A.0x0000000000000000.CallbackScheduler.CallbackProcessed"),
		TransactionID:    unittest.IdentifierFixture(),
		TransactionIndex: 0,
		EventIndex:       0,
		Payload:          payload,
	}
}

func createInvalidTypeEvent() flow.Event {
	return flow.Event{
		Type:             flow.EventType("A.0x0000000000000000.SomeContract.WrongEvent"),
		TransactionID:    unittest.IdentifierFixture(),
		TransactionIndex: 0,
		EventIndex:       0,
		Payload:          []byte("invalid"),
	}
}

func createInvalidPayloadEvent() flow.Event {
	return flow.Event{
		Type:             flow.EventType("A.0x0000000000000000.CallbackScheduler.CallbackProcessed"),
		TransactionID:    unittest.IdentifierFixture(),
		TransactionIndex: 0,
		EventIndex:       0,
		Payload:          []byte("not valid ccf"),
	}
}
