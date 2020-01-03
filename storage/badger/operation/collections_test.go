// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestCollectionGuaranteesInsertRetrieve(t *testing.T) {

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		hash := crypto.Hash{0x13, 0x37}
		expected := []*flow.CollectionGuarantee{
			{CollectionHash: crypto.Hash{0x01}, Signatures: []crypto.Signature{{0x10}}},
			{CollectionHash: crypto.Hash{0x02}, Signatures: []crypto.Signature{{0x20}}},
			{CollectionHash: crypto.Hash{0x03}, Signatures: []crypto.Signature{{0x30}}},
		}

		err := db.Update(InsertCollectionGuarantees(hash, expected))
		require.Nil(t, err)

		var actual []*flow.CollectionGuarantee
		err = db.View(RetrieveCollectionGuarantees(hash, &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})
}

func TestCollectionsInsertRetrieve(t *testing.T) {

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		hash := crypto.Hash{0x13, 0x37}
		expected := []*flow.CollectionGuarantee{
			{CollectionHash: crypto.Hash{0x01}, Signatures: []crypto.Signature{{0x10}}},
			{CollectionHash: crypto.Hash{0x02}, Signatures: []crypto.Signature{{0x20}}},
			{CollectionHash: crypto.Hash{0x03}, Signatures: []crypto.Signature{{0x30}}},
		}

		err := db.Update(InsertCollectionGuarantees(hash, expected))
		require.Nil(t, err)

		var actual []*flow.CollectionGuarantee
		err = db.View(RetrieveCollectionGuarantees(hash, &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})
}
