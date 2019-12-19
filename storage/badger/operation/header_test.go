// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestHeaderInsertNewRetrieve(t *testing.T) {

	unittest.RunWithDB(t, func(db *badger.DB) {
		expected := flow.Header{
			Number:     1337,
			Timestamp:  time.Now().UTC(),
			Parent:     crypto.Hash([]byte("parent")),
			Payload:    crypto.Hash([]byte("payload")),
			Signatures: []crypto.Signature{{0x99}},
		}
		hash := expected.Hash()

		err := db.Update(InsertNewHeader(&expected))
		require.Nil(t, err)

		var actual flow.Header
		err = db.View(RetrieveHeader(hash, &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})
}
