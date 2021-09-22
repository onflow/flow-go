package badger_test

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestInitPublic(t *testing.T) {
	unittest.RunWithTypedBadgerDB(t, bstorage.InitPublic, func(db *badger.DB) {
		err := operation.EnsurePublicDB(db)
		require.NoError(t, err)
		err = operation.EnsureSecretDB(db)
		require.Error(t, err)
	})
}

func TestInitSecret(t *testing.T) {
	unittest.RunWithTypedBadgerDB(t, bstorage.InitSecret, func(db *badger.DB) {
		err := operation.EnsureSecretDB(db)
		require.NoError(t, err)
		err = operation.EnsurePublicDB(db)
		require.Error(t, err)
	})
}
