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

type eventWrapper struct {
	TxID             []byte
	Index            uint32
	Type             string
	TransactionIndex uint32
	Payload          []byte
}

func wrapEvent(e flow.Event) eventWrapper {
	return eventWrapper{
		TxID:             e.TransactionID[:],
		Index:            e.EventIndex,
		Type:             string(e.Type),
		TransactionIndex: e.TransactionIndex,
		Payload:          e.Payload,
	}
}

func TestEventFingerprint(t *testing.T) {
	evt := unittest.EventFixture(flow.EventAccountCreated, 13, 12, unittest.IdentifierFixture(), 32)

	data := fingerprint.Fingerprint(evt)
	var decoded eventWrapper
	rlp.NewMarshaler().MustUnmarshal(data, &decoded)
	assert.Equal(t, wrapEvent(evt), decoded)
}

func TestEventID(t *testing.T) {

	// EventID was historically calculated from just TxID and eventIndex which are enough to uniquely identify it in a system
	// This test ensures we don't break this promise while introducing proper fingerprinting (which accounts for all the fields)

	txID := unittest.IdentifierFixture()
	evtA := unittest.EventFixture(flow.EventAccountUpdated, 21, 37, txID, 2)
	evtB := unittest.EventFixture(flow.EventAccountCreated, 0, 37, txID, 22)

	evtC := unittest.EventFixture(evtA.Type, evtA.TransactionIndex, evtA.EventIndex+1, txID, 2)
	evtC.Payload = evtA.Payload

	a := evtA.ID()
	b := evtB.ID()
	c := evtC.ID()

	assert.Equal(t, a, b)
	assert.NotEqual(t, a, c)
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

	expectedRootHashHex := "355446d7b2b9653403abe28ccc405f46c059d2059cb7863f4964c401ee1aa83b"

	ABHash, err := flow.EventsMerkleRootHash([]flow.Event{eventA, eventB})
	assert.NoError(t, err)
	assert.Equal(t, expectedRootHashHex, ABHash.String())
}

func TestEmptyEventsMerkleRootHash(t *testing.T) {
	actualHash, err := flow.EventsMerkleRootHash([]flow.Event{})
	require.NoError(t, err)
	require.Equal(t, flow.EmptyEventCollectionID, actualHash)
}
