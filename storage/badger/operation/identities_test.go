// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
)

func TestIdentitiesInsertRetrieve(t *testing.T) {

	dir := filepath.Join(os.TempDir(), fmt.Sprintf("flow-test-db-%d", rand.Uint64()))
	defer os.RemoveAll(dir)
	db, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil))
	require.Nil(t, err)

	hash := crypto.Hash{0x13, 0x37}
	expected := flow.IdentityList{
		{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.Role(1), Stake: 1},
		{NodeID: flow.Identifier{0x02}, Address: "a2", Role: flow.Role(2), Stake: 2},
		{NodeID: flow.Identifier{0x03}, Address: "a3", Role: flow.Role(3), Stake: 3},
	}

	err = db.Update(InsertIdentities(hash, expected))
	require.Nil(t, err)

	var actual flow.IdentityList
	err = db.View(RetrieveIdentities(hash, &actual))
	require.Nil(t, err)

	assert.Equal(t, expected, actual)
}
