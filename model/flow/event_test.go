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
	evt := unittest.EventFixture(flow.EventAccountCreated, 13, 12, unittest.IdentifierFixture(), 32)

	data := fingerprint.Fingerprint(evt)
	var decoded flow.Event
	rlp.NewMarshaler().MustUnmarshal(data, &decoded)
	assert.Equal(t, evt, decoded)
}

// TestEventMalleability checks that Event is not malleable: any change in its data
// should result in a different ID.
func TestSealMalleability(t *testing.T) {
	txID := unittest.IdentifierFixture()
	event := unittest.EventFixture(flow.EventAccountUpdated, 21, 37, txID, 2)

	unittest.RequireEntityNonMalleable(t, &event)
}

func TestEventsList(t *testing.T) {

	eventA := unittest.EventFixture(flow.EventAccountUpdated, 21, 37, unittest.IdentifierFixture(), 2)
	eventB := unittest.EventFixture(flow.EventAccountCreated, 0, 37, unittest.IdentifierFixture(), 22)
	eventC := unittest.EventFixture(flow.EventAccountCreated, 0, 37, unittest.IdentifierFixture(), 22)

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
