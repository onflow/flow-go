package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/encoding/rlp"
	"github.com/onflow/flow-go/model/fingerprint"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestEventFingerprint verifies that the Fingerprint function produces
// a consistent RLP-encoded representation of an Event. It ensures that
// decoding the fingerprint results in a correctly ordered structure.
func TestEventFingerprint(t *testing.T) {
	evt := unittest.EventFixture()

	data := fingerprint.Fingerprint(evt)
	var decoded flow.Event
	rlp.NewMarshaler().MustUnmarshal(data, &decoded)
	assert.Equal(t, evt, decoded)
}

// TestEventMalleability checks that Event is not malleable: any change in its data
// should result in a different ID.
func TestEventMalleability(t *testing.T) {
	event := unittest.EventFixture()

	unittest.RequireEntityNonMalleable(t, &event)
}

func TestEventsList(t *testing.T) {
	eventA := unittest.EventFixture()
	eventB := unittest.EventFixture()
	eventC := unittest.EventFixture()

	listAB := flow.EventsList{
		eventA,
		eventB,
	}

	listBA := flow.EventsList{
		eventB,
		eventA,
	}

	listAC := flow.EventsList{
		eventA,
		eventC,
	}

	ABHash, err := flow.EventsMerkleRootHash(listAB)
	assert.NoError(t, err)
	ACHash, err := flow.EventsMerkleRootHash(listAC)
	assert.NoError(t, err)
	BAHash, err := flow.EventsMerkleRootHash(listBA)
	assert.NoError(t, err)

	t.Run("different events have different hash", func(t *testing.T) {
		assert.NotEqual(t, ABHash, ACHash)
	})

	t.Run("insertion order does not matter", func(t *testing.T) {
		assert.Equal(t, ABHash, BAHash)
	})
}

func TestEventsMerkleRootHash(t *testing.T) {
	eventA := flow.Event{
		Type:             "eventTypeString",
		TransactionIndex: 1,
		EventIndex:       2,
		Payload:          []byte("cadence-json encoded data"),
		TransactionID:    [flow.IdentifierLen]byte{1, 2, 3},
	}

	eventB := flow.Event{
		Type:             "eventTypeString",
		TransactionIndex: 1,
		EventIndex:       3,
		Payload:          []byte("cadence-json encoded data"),
		TransactionID:    [flow.IdentifierLen]byte{1, 2, 3},
	}

	expectedRootHashHex := "c53a6592de573a24547b616172abd9131651d6b7d829e5694a25fa183db7ae01"

	ABHash, err := flow.EventsMerkleRootHash([]flow.Event{eventA, eventB})
	assert.NoError(t, err)
	assert.Equal(t, expectedRootHashHex, ABHash.String())
}

func TestEmptyEventsMerkleRootHash(t *testing.T) {
	actualHash, err := flow.EventsMerkleRootHash([]flow.Event{})
	require.NoError(t, err)
	require.Equal(t, flow.EmptyEventCollectionID, actualHash)
}

// TestNewEvent verifies the behavior of the NewEvent constructor.
// It ensures proper handling of both valid and invalid untrusted input fields.
//
// Test Cases:
//
// 1. Valid input:
//   - Verifies that a properly populated UntrustedEvent results in a valid Event.
//
// 2. Invalid input with empty event type:
//   - Ensures an error is returned when the Type field is an empty string.
//
// 3. Invalid input with zero transaction ID:
//   - Ensures an error is returned when the TransactionID is zero.
//
// 4. Invalid input with nil Payload:
//   - Ensures an error is returned when the Payload field is nil.
//
// 5. Invalid input with empty Payload:
//   - Ensures an error is returned when the Payload field is an empty byte slice.
func TestNewEvent(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		event, err := flow.NewEvent(
			flow.UntrustedEvent{
				Type:             flow.EventAccountCreated,
				TransactionID:    unittest.IdentifierFixture(),
				TransactionIndex: 1,
				EventIndex:       1,
				Payload:          []byte("cadence-json encoded data"),
			},
		)
		require.NoError(t, err)
		require.NotNil(t, event)
	})

	t.Run("invalid input, type is empty", func(t *testing.T) {
		event, err := flow.NewEvent(
			flow.UntrustedEvent{
				Type:             "",
				TransactionID:    unittest.IdentifierFixture(),
				TransactionIndex: 1,
				EventIndex:       1,
				Payload:          []byte("cadence-json encoded data"),
			},
		)
		require.Error(t, err)
		require.Nil(t, event)
		assert.Contains(t, err.Error(), "event type must not be empty")
	})

	t.Run("invalid input, transaction ID is zero", func(t *testing.T) {
		event, err := flow.NewEvent(
			flow.UntrustedEvent{
				Type:             flow.EventAccountCreated,
				TransactionID:    flow.ZeroID,
				TransactionIndex: 1,
				EventIndex:       1,
				Payload:          []byte("cadence-json encoded data"),
			},
		)
		require.Error(t, err)
		require.Nil(t, event)
		assert.Contains(t, err.Error(), "transaction ID must not be zero")
	})

	t.Run("invalid input with nil payload", func(t *testing.T) {
		event, err := flow.NewEvent(
			flow.UntrustedEvent{
				Type:             flow.EventAccountCreated,
				TransactionID:    unittest.IdentifierFixture(),
				TransactionIndex: 1,
				EventIndex:       1,
				Payload:          nil,
			},
		)
		require.Error(t, err)
		require.Nil(t, event)
		assert.Contains(t, err.Error(), "payload must not be empty")
	})

	t.Run("invalid input with empty payload", func(t *testing.T) {
		event, err := flow.NewEvent(
			flow.UntrustedEvent{
				Type:             flow.EventAccountCreated,
				TransactionID:    unittest.IdentifierFixture(),
				TransactionIndex: 1,
				EventIndex:       1,
				Payload:          []byte{},
			},
		)
		require.Error(t, err)
		require.Nil(t, event)
		assert.Contains(t, err.Error(), "payload must not be empty")
	})
}
