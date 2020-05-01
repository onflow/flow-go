package procedure

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func saveAndRetrieve(t *testing.T, db *badger.DB, header *flow.Header, payload *flow.Payload, retrievedPayload *flow.Payload) {

	header.PayloadHash = payload.Hash()
	err := db.Update(operation.InsertHeader(header))
	require.NoError(t, err)

	err = db.Update(InsertPayload(payload))
	require.NoError(t, err)

	err = db.Update(IndexPayload(header, payload))
	require.NoError(t, err)

	err = db.View(RetrievePayload(header.ID(), retrievedPayload))
	require.NoError(t, err)
}

func check(t *testing.T, db *badger.DB, header *flow.Header, payload *flow.Payload) {

	var retrievedPayload flow.Payload

	saveAndRetrieve(t, db, header, payload, &retrievedPayload)

	assert.Equal(t, *payload, retrievedPayload)
	assert.Equal(t, payload.Hash(), retrievedPayload.Hash())
}

func TestPayloadNilSeals(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		block := unittest.BlockFixture()
		block.Payload.Seals = nil

		check(t, db, &block.Header, &block.Payload)
	})
}

func TestPayloadEmptySeals(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		block := unittest.BlockFixture()
		block.Payload.Seals = make([]*flow.Seal, 0)

		var retrievedPayload flow.Payload
		saveAndRetrieve(t, db, &block.Header, &block.Payload, &retrievedPayload)

		//check(t, db, &block.Header, &block.Payload)
		assert.Equal(t, block.Payload.Hash(), retrievedPayload.Hash())
		assert.Nil(t, retrievedPayload.Seals)
	})
}

func TestPayloadOrderedSeals(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		block := unittest.BlockFixture()

		seal1 := unittest.BlockSealFixture()
		seal2 := unittest.BlockSealFixture()
		seal3 := unittest.BlockSealFixture()

		block.Payload.Seals = []*flow.Seal{
			seal1, seal2, seal3,
		}

		check(t, db, &block.Header, &block.Payload)
	})
}

func TestPayloadNilGuarantees(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		block := unittest.BlockFixture()
		block.Payload.Guarantees = nil

		check(t, db, &block.Header, &block.Payload)
	})
}

func TestPayloadEmptyGuarantees(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		block := unittest.BlockFixture()
		block.Payload.Guarantees = make([]*flow.CollectionGuarantee, 0)

		var retrievedPayload flow.Payload
		saveAndRetrieve(t, db, &block.Header, &block.Payload, &retrievedPayload)

		//check(t, db, &block.Header, &block.Payload)
		assert.Equal(t, block.Payload.Hash(), retrievedPayload.Hash())
		assert.Nil(t, retrievedPayload.Guarantees)
	})
}

func TestPayloadOrderedGuarantees(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		block := unittest.BlockFixture()
		block.Payload.Guarantees = unittest.CollectionGuaranteesFixture(5)

		check(t, db, &block.Header, &block.Payload)
	})
}

func TestPayloadNilIdentities(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		block := unittest.BlockFixture()
		block.Payload.Identities = nil

		check(t, db, &block.Header, &block.Payload)
	})
}

func TestPayloadEmptyIdentities(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		block := unittest.BlockFixture()
		block.Payload.Identities = make([]*flow.Identity, 0)

		var retrievedPayload flow.Payload
		saveAndRetrieve(t, db, &block.Header, &block.Payload, &retrievedPayload)

		//check(t, db, &block.Header, &block.Payload)
		assert.Equal(t, block.Payload.Hash(), retrievedPayload.Hash())
		assert.Nil(t, retrievedPayload.Identities)
	})
}
