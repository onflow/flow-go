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

func TestHeaderInsertCheckRetrieve(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		expected := flow.Header{
			View:                  1337,
			Timestamp:             time.Now().UTC(),
			ParentID:              flow.Identifier{0x11},
			PayloadHash:           flow.Identifier{0x22},
			ProposerID:            flow.Identifier{0x33},
			ParentStakingSigs:     []crypto.Signature{[]byte{0x88}},
			ParentRandomBeaconSig: crypto.Signature{0x77},
			ParentSigners:         []flow.Identifier{flow.Identifier{0x44}},
		}
		blockID := expected.ID()

		err := db.Update(InsertHeader(&expected))
		require.Nil(t, err)

		var exists bool
		err = db.View(CheckHeader(blockID, &exists))
		require.Nil(t, err)
		require.True(t, exists)

		var actual flow.Header
		err = db.View(RetrieveHeader(blockID, &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, actual)
	})
}
