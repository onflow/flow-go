package pebble_test

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/require"

	bstorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/storage/pebble/operation"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestInitPublic(t *testing.T) {
	unittest.RunWithTypedPebbleDB(t, bstorage.InitPublic, func(db *pebble.DB) {
		err := operation.EnsurePublicDB(db)
		require.NoError(t, err)
		err = operation.EnsureSecretDB(db)
		require.Error(t, err)
	})
}

func TestInitSecret(t *testing.T) {
	unittest.RunWithTypedPebbleDB(t, bstorage.InitSecret, func(db *pebble.DB) {
		err := operation.EnsureSecretDB(db)
		require.NoError(t, err)
		err = operation.EnsurePublicDB(db)
		require.Error(t, err)
	})
}
