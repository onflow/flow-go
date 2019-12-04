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

	"github.com/dapperlabs/flow-go/model/flow"
)

func TestAddressInsertRetrieve(t *testing.T) {

	dir := filepath.Join(os.TempDir(), fmt.Sprintf("flow-test-db-%d", rand.Uint64()))
	db, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil))
	require.Nil(t, err)

	nodeID := flow.Identifier{0x01}
	expected := "address"

	err = db.Update(InsertAddress(nodeID, expected))
	require.Nil(t, err)

	var actual string
	err = db.View(RetrieveAddress(nodeID, &actual))
	require.Nil(t, err)

	assert.Equal(t, expected, actual)
}
