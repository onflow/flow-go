package badger_test

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/dgraph-io/badger/v2"
	errors2 "github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/storage"
	badger2 "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func setUp(t *testing.T) *badger.DB {
	dir := filepath.Join(os.TempDir(), fmt.Sprintf("flow-test-db-%d", rand.Uint64()))
	db, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil))
	require.Nil(t, err)
	return db
}

func TestRetrievalByNumber(t *testing.T) {

	db := setUp(t)

	blocks := badger2.NewBlocks(db)

	block := unittest.BlockFixture()
	block.Number = 21

	err := blocks.Save(&block)
	require.NoError(t, err)

	byNumber, err := blocks.ByNumber(21)
	require.NoError(t, err)

	assert.Equal(t, byNumber, &block)
}

func TestRetrievalByNonexistingNumber(t *testing.T) {

	db := setUp(t)

	blocks := badger2.NewBlocks(db)

	block := unittest.BlockFixture()
	block.Number = 21

	err := blocks.Save(&block)
	require.NoError(t, err)

	_, err = blocks.ByNumber(37)

	assert.Equal(t, storage.NotFoundErr, err)
}

func TestStoringSameBlockTwice(t *testing.T) {

	db := setUp(t)

	blocks := badger2.NewBlocks(db)

	block := unittest.BlockFixture()
	block.Number = 21

	err := blocks.Save(&block)
	require.NoError(t, err)

	err = blocks.Save(&block)
	require.NoError(t, err)
}

func TestStoringBlockWithDifferentDateButSameNumberTwice(t *testing.T) {

	db := setUp(t)

	blocks := badger2.NewBlocks(db)

	block := unittest.BlockFixture()
	block.Number = 21

	err := blocks.Save(&block)
	require.NoError(t, err)

	block2 := block
	block2.Signatures = []crypto.Signature{[]byte("magic")}

	err = blocks.Save(&block2)
	realError := errors2.Cause(err)

	require.Equal(t, storage.DifferentDataErr, realError)
}
