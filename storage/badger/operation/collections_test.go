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
	"github.com/dapperlabs/flow-go/model/collection"
)

func TestCollectionsInsertRetrieve(t *testing.T) {

	dir := filepath.Join(os.TempDir(), fmt.Sprintf("flow-test-db-%d", rand.Uint64()))
	defer os.RemoveAll(dir)
	db, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil))
	require.Nil(t, err)

	hash := crypto.Hash{0x13, 0x37}
	expected := []*collection.GuaranteedCollection{
		{CollectionHash: crypto.Hash{0x01}, Signatures: []crypto.Signature{{0x10}}},
		{CollectionHash: crypto.Hash{0x02}, Signatures: []crypto.Signature{{0x20}}},
		{CollectionHash: crypto.Hash{0x03}, Signatures: []crypto.Signature{{0x30}}},
	}

	err = db.Update(InsertNewCollections(hash, expected))
	require.Nil(t, err)

	var actual []*collection.GuaranteedCollection
	err = db.View(RetrieveCollections(hash, &actual))
	require.Nil(t, err)

	assert.Equal(t, expected, actual)
}
