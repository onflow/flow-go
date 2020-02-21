package procedure

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func saveAndRetrieve(t *testing.T, db *badger.DB, payload *flow.Payload, retrievedPayload *flow.Payload) {
	err := db.Update(InsertPayload(payload))
	require.NoError(t, err)

	err = db.Update(IndexPayload(payload))
	require.NoError(t, err)

	err = db.View(RetrievePayload(payload.Hash(), retrievedPayload))
	require.NoError(t, err)
}

func check(t *testing.T, db *badger.DB, payload *flow.Payload) {

	var retrievedPayload flow.Payload

	saveAndRetrieve(t, db, payload, &retrievedPayload)

	assert.Equal(t, *payload, retrievedPayload)
	assert.Equal(t, payload.Hash(), retrievedPayload.Hash())
}

func TestPayloadNilSeals(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		payload := unittest.BlockFixture().Payload
		payload.Seals = nil

		check(t, db, &payload)
	})
}

func TestPayloadEmptySeals(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		payload := unittest.BlockFixture().Payload
		payload.Seals = make([]*flow.Seal, 0)

		var retrievedPayload flow.Payload
		saveAndRetrieve(t, db, &payload, &retrievedPayload)

		//check(t, db, &payload)
		assert.Equal(t, payload.Hash(), retrievedPayload.Hash())
		assert.Nil(t, retrievedPayload.Seals)
	})
}

func TestPayloadOrderedSeals(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		payload := unittest.BlockFixture().Payload

		seal1 := unittest.SealFixture()
		seal2 := unittest.SealFixture()
		seal3 := unittest.SealFixture()

		payload.Seals = []*flow.Seal{
			&seal1, &seal2, &seal3,
		}

		check(t, db, &payload)
	})
}

func TestPayloadNilGuarantees(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		payload := unittest.BlockFixture().Payload
		payload.Guarantees = nil

		check(t, db, &payload)
	})
}

func TestPayloadEmptyGuarantees(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		payload := unittest.BlockFixture().Payload
		payload.Guarantees = make([]*flow.CollectionGuarantee, 0)

		var retrievedPayload flow.Payload
		saveAndRetrieve(t, db, &payload, &retrievedPayload)

		//check(t, db, &payload)
		assert.Equal(t, payload.Hash(), retrievedPayload.Hash())
		assert.Nil(t, retrievedPayload.Guarantees)
	})
}

func TestPayloadOrderedGuarantees(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		payload := unittest.BlockFixture().Payload
		payload.Guarantees = unittest.CollectionGuaranteesFixture(5)

		check(t, db, &payload)
	})
}

func TestPayloadNilIdentities(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		payload := unittest.BlockFixture().Payload
		payload.Identities = nil

		check(t, db, &payload)
	})
}

func TestPayloadEmptyIdentities(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		payload := unittest.BlockFixture().Payload
		payload.Identities = make([]*flow.Identity, 0)

		var retrievedPayload flow.Payload
		saveAndRetrieve(t, db, &payload, &retrievedPayload)

		//check(t, db, &payload)
		assert.Equal(t, payload.Hash(), retrievedPayload.Hash())
		assert.Nil(t, retrievedPayload.Identities)
	})
}

func TestPayloadOrderedIdentities(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		payload := unittest.BlockFixture().Payload
		payload.Identities = unittest.IdentityListFixture(5)

		check(t, db, &payload)
	})
}
