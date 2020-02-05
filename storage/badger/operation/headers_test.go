// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestHeaderInsertRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		expected := flow.Header{
			Number:      1337,
			Timestamp:   time.Now().UTC(),
			ParentID:    flow.Identifier{0x11},
			PayloadHash: flow.Identifier{0x22},
			ProposerID:  flow.Identifier{0x33},
			ParentSig: flow.AggregatedSignature{
				Raw:     []byte{0x44},
				Signers: []bool{true},
			},
		}
		blockID := expected.ID()

		err := db.Update(InsertHeader(&expected))
		require.Nil(t, err)

		var actual flow.Header
		err = db.View(RetrieveHeader(blockID, &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})
}
