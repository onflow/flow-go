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

func TestGuaranteedCollectionsInsertRetrieve(t *testing.T) {

	unittest.RunWithDB(t, func(db *badger.DB) {
		hash := crypto.Hash{0x13, 0x37}
		expected := []*collection.GuaranteedCollection{
			{CollectionHash: crypto.Hash{0x01}, Signatures: []crypto.Signature{{0x10}}},
			{CollectionHash: crypto.Hash{0x02}, Signatures: []crypto.Signature{{0x20}}},
			{CollectionHash: crypto.Hash{0x03}, Signatures: []crypto.Signature{{0x30}}},
		}

		err := db.Update(InsertNewGuaranteedCollections(hash, expected))
		require.Nil(t, err)

		var actual []*flow.GuaranteedCollection
		err = db.View(RetrieveGuaranteedCollections(hash, &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})
}

func TestFlowCollectionsInsertRetrieve(t *testing.T) {

	unittest.RunWithDB(t, func(db *badger.DB) {
		hash := crypto.Hash{0x13, 0x37}
		expected := []*flow.GuaranteedCollection{
			{CollectionHash: crypto.Hash{0x01}, Signatures: []crypto.Signature{{0x10}}},
			{CollectionHash: crypto.Hash{0x02}, Signatures: []crypto.Signature{{0x20}}},
			{CollectionHash: crypto.Hash{0x03}, Signatures: []crypto.Signature{{0x30}}},
		}

		err := db.Update(InsertNewGuaranteedCollections(hash, expected))
		require.Nil(t, err)

		var actual []*flow.GuaranteedCollection
		err = db.View(RetrieveGuaranteedCollections(hash, &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})
}
