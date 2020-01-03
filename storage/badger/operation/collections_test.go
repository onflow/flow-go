// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

func TestIndexGuaranteedCollectionByBlockHashInsertRetrieve(t *testing.T) {

	dir := filepath.Join(os.TempDir(), fmt.Sprintf("flow-test-db-%d", rand.Uint64()))
	defer os.RemoveAll(dir)
	db, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil))
	require.Nil(t, err)

	blockHash := crypto.Hash{0x12, 0x34}
	expected := []*flow.GuaranteedCollection{
		{CollectionHash: crypto.Hash{0x01}, Signatures: []crypto.Signature{{0x10}}},
		{CollectionHash: crypto.Hash{0x02}, Signatures: []crypto.Signature{{0x20}}},
	}

	err = db.Update(func(tx *badger.Txn) error {
		for _, coll := range expected {
			if err := InsertGuaranteedCollection(coll.Fingerprint(), coll)(tx); err != nil {
				return err
			}
			if err := IndexGuaranteedCollectionByBlockHash(blockHash, coll)(tx); err != nil {
				return err
			}
		}
		return nil
	})
	require.Nil(t, err)

	var actual []*flow.GuaranteedCollection
	err = db.View(RetrieveGuaranteedCollectionsByBlockHash(blockHash, &actual))
	require.Nil(t, err)

	assert.Equal(t, expected, actual)
}

func TestIndexGuaranteedCollectionByBlockHashMultipleBlocks(t *testing.T) {
	dir := filepath.Join(os.TempDir(), fmt.Sprintf("flow-test-db-%d", rand.Uint64()))
	defer os.RemoveAll(dir)
	db, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil))
	require.Nil(t, err)

	block1Hash := crypto.Hash{0x10}
	block2Hash := crypto.Hash{0x20}
	expected1 := []*flow.GuaranteedCollection{
		{CollectionHash: crypto.Hash{0x01}, Signatures: []crypto.Signature{{0x1}}},
	}
	expected2 := []*flow.GuaranteedCollection{
		{CollectionHash: crypto.Hash{0x02}, Signatures: []crypto.Signature{{0x2}}},
	}

	// insert block 1
	err = db.Update(func(tx *badger.Txn) error {
		for _, coll := range expected1 {
			if err := InsertGuaranteedCollection(coll.Fingerprint(), coll)(tx); err != nil {
				return err
			}
			if err := IndexGuaranteedCollectionByBlockHash(block1Hash, coll)(tx); err != nil {
				return err
			}
		}
		return nil
	})
	require.Nil(t, err)

	// insert block 2
	err = db.Update(func(tx *badger.Txn) error {
		for _, coll := range expected2 {
			if err := InsertGuaranteedCollection(coll.Fingerprint(), coll)(tx); err != nil {
				return err
			}
			if err := IndexGuaranteedCollectionByBlockHash(block2Hash, coll)(tx); err != nil {
				return err
			}
		}
		return nil
	})
	require.Nil(t, err)

	t.Run("should retrieve collections for block", func(t *testing.T) {
		var actual1 []*flow.GuaranteedCollection
		err = db.View(RetrieveGuaranteedCollectionsByBlockHash(block1Hash, &actual1))
		assert.NoError(t, err)
		assert.Equal(t, expected1, actual1)

		// get block 2
		var actual2 []*flow.GuaranteedCollection
		err = db.View(RetrieveGuaranteedCollectionsByBlockHash(block2Hash, &actual2))
		assert.NoError(t, err)
		assert.Equal(t, expected1, actual1)
	})
}

func TestCollections(t *testing.T) {
	dir := filepath.Join(os.TempDir(), fmt.Sprintf("flow-test-db-%d", rand.Uint64()))
	defer os.RemoveAll(dir)
	db, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil))
	require.Nil(t, err)

	expected := flow.Collection{
		Transactions: []flow.Fingerprint{[]byte{1}, []byte{2}},
	}

	t.Run("Retrieve nonexistant", func(t *testing.T) {
		var actual flow.Collection
		err = db.View(RetrieveCollection(expected.Fingerprint(), &actual))
		assert.Error(t, err)
	})

	t.Run("Insert", func(t *testing.T) {
		err := db.Update(InsertCollection(expected.Fingerprint(), &expected))
		require.NoError(t, err)

		var actual flow.Collection
		err = db.View(RetrieveCollection(expected.Fingerprint(), &actual))
		assert.NoError(t, err)

		assert.Equal(t, expected, actual)
	})

	t.Run("Remove", func(t *testing.T) {
		err := db.Update(RemoveCollection(expected.Fingerprint()))
		require.NoError(t, err)

		var actual flow.Collection
		err = db.View(RetrieveCollection(expected.Fingerprint(), &actual))
		assert.Error(t, err)
	})
}
