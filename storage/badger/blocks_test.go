package badger_test

import (
	"errors"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/storage"
	badger2 "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestBlockRetrievalByNumber(t *testing.T) {

	unittest.RunWithDB(t, func(db *badger.DB) {
		blocks := badger2.NewBlocks(db)

		block := unittest.BlockFixture()
		block.Number = 21

		err := blocks.Save(&block)
		require.NoError(t, err)

		byNumber, err := blocks.ByNumber(21)
		require.NoError(t, err)

		assert.Equal(t, byNumber, &block)
	})
}

func TestBlockRetrievalByNonexistingNumber(t *testing.T) {

	unittest.RunWithDB(t, func(db *badger.DB) {

		blocks := badger2.NewBlocks(db)

		block := unittest.BlockFixture()
		block.Number = 21

		err := blocks.Save(&block)
		require.NoError(t, err)

		_, err = blocks.ByNumber(37)

		assert.Equal(t, storage.NotFoundErr, err)
	})
}

func TestStoringSameBlockTwice(t *testing.T) {

	unittest.RunWithDB(t, func(db *badger.DB) {

		blocks := badger2.NewBlocks(db)

		block := unittest.BlockFixture()
		block.Number = 21

		err := blocks.Save(&block)
		require.NoError(t, err)

		err = blocks.Save(&block)
		require.NoError(t, err)
	})
}

func TestStoringBlockWithDifferentDateButSameNumberTwice(t *testing.T) {

	unittest.RunWithDB(t, func(db *badger.DB) {

		blocks := badger2.NewBlocks(db)

		block := unittest.BlockFixture()
		block.Number = 21

		err := blocks.Save(&block)
		require.NoError(t, err)

		block2 := block
		block2.Signatures = []crypto.Signature{[]byte("magic")}

		err = blocks.Save(&block2)

		realErr := errors.Unwrap(err)

		require.Equal(t, storage.DifferentDataErr, realErr)
	})
}
