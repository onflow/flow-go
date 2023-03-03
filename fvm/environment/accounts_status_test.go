package environment_test

import (
	"bytes"
	"testing"

	"github.com/onflow/atree"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/environment"
)

func TestAccountStatus(t *testing.T) {

	s := environment.NewAccountStatus()
	require.False(t, s.IsAccountFrozen())

	t.Run("test frozen flag set/reset", func(t *testing.T) {
		// TODO: remove freezing feature
		t.Skip("Skip as we are removing the freezing feature.")

		s.SetFrozenFlag(true)
		require.True(t, s.IsAccountFrozen())

		s.SetFrozenFlag(false)
		require.False(t, s.IsAccountFrozen())
	})

	t.Run("test setting values", func(t *testing.T) {
		index := atree.StorageIndex{1, 2, 3, 4, 5, 6, 7, 8}
		s.SetStorageIndex(index)
		s.SetPublicKeyCount(34)
		s.SetStorageUsed(56)

		require.Equal(t, uint64(56), s.StorageUsed())
		returnedIndex := s.StorageIndex()
		require.True(t, bytes.Equal(index[:], returnedIndex[:]))
		require.Equal(t, uint64(34), s.PublicKeyCount())

	})

	t.Run("test serialization", func(t *testing.T) {
		b := append([]byte(nil), s.ToBytes()...)
		clone, err := environment.AccountStatusFromBytes(b)
		require.NoError(t, err)
		require.Equal(t, s.IsAccountFrozen(), clone.IsAccountFrozen())
		require.Equal(t, s.StorageIndex(), clone.StorageIndex())
		require.Equal(t, s.PublicKeyCount(), clone.PublicKeyCount())
		require.Equal(t, s.StorageUsed(), clone.StorageUsed())

		// invalid size bytes
		_, err = environment.AccountStatusFromBytes([]byte{1, 2})
		require.Error(t, err)
	})
}
