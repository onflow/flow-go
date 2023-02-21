package operation

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestInsertQuorumCertificate(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		expected := unittest.QuorumCertificateFixture()

		err := db.Update(InsertQuorumCertificate(expected))
		require.Nil(t, err)

		var actual flow.QuorumCertificate
		err = db.View(RetrieveQuorumCertificate(expected.BlockID, &actual))
		require.Nil(t, err)

		assert.Equal(t, expected, &actual)
	})
}
