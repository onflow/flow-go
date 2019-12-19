// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

func TestHeaderInsertNewRetrieve(t *testing.T) {

	dir := filepath.Join(os.TempDir(), fmt.Sprintf("flow-test-db-%d", rand.Uint64()))
	defer os.RemoveAll(dir)
	db, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil))
	require.Nil(t, err)

	expected := flow.Header{
		Number:     1337,
		Timestamp:  time.Now().UTC(),
		Parent:     crypto.Hash([]byte("parent")),
		Payload:    crypto.Hash([]byte("payload")),
		Signatures: []crypto.Signature{{0x99}},
	}
	hash := expected.Hash()

	err = db.Update(InsertNewHeader(&expected))
	require.Nil(t, err)

	var actual flow.Header
	err = db.View(RetrieveHeader(hash, &actual))
	require.Nil(t, err)

	assert.Equal(t, expected, actual)
}
