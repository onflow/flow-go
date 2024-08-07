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

	t.Run("test setting values", func(t *testing.T) {
		index := atree.SlabIndex{1, 2, 3, 4, 5, 6, 7, 8}
		s.SetStorageIndex(index)
		s.SetPublicKeyCount(34)
		s.SetStorageUsed(56)
		s.SetAccountIdCounter(78)

		require.Equal(t, uint64(56), s.StorageUsed())
		returnedIndex := s.SlabIndex()
		require.True(t, bytes.Equal(index[:], returnedIndex[:]))
		require.Equal(t, uint32(34), s.PublicKeyCount())
		require.Equal(t, uint64(78), s.AccountIdCounter())

	})

	t.Run("test serialization", func(t *testing.T) {
		b := append([]byte(nil), s.ToBytes()...)
		clone, err := environment.AccountStatusFromBytes(b)
		require.NoError(t, err)
		require.Equal(t, s.SlabIndex(), clone.SlabIndex())
		require.Equal(t, s.PublicKeyCount(), clone.PublicKeyCount())
		require.Equal(t, s.StorageUsed(), clone.StorageUsed())
		require.Equal(t, s.AccountIdCounter(), clone.AccountIdCounter())

		// invalid size bytes
		_, err = environment.AccountStatusFromBytes([]byte{1, 2})
		require.Error(t, err)
	})

	t.Run("test serialization - v1 format", func(t *testing.T) {
		// TODO: remove this test when we remove support for the old format
		oldBytes := []byte{
			0,                      // flags
			0, 0, 0, 0, 0, 0, 0, 7, // storage used
			0, 0, 0, 0, 0, 0, 0, 6, // storage index
			0, 0, 0, 0, 0, 0, 0, 5, // public key counts
		}

		// The new format has an extra 8 bytes for the account id counter
		// so we need to increase the storage used by 8 bytes while migrating it
		// for v2->v3 migration, we need to decrease the storage used by 4 bytes
		increaseInSize := uint64(4)

		migrated, err := environment.AccountStatusFromBytes(oldBytes)
		require.NoError(t, err)
		require.Equal(t, atree.SlabIndex{0, 0, 0, 0, 0, 0, 0, 6}, migrated.SlabIndex())
		require.Equal(t, uint32(5), migrated.PublicKeyCount())
		require.Equal(t, uint64(7)+increaseInSize, migrated.StorageUsed())
		require.Equal(t, uint64(0), migrated.AccountIdCounter())
	})

	t.Run("test serialization - v2 format", func(t *testing.T) {
		// TODO: remove this test when we remove support for the old format
		oldBytes := []byte{
			0,                      // flags
			0, 0, 0, 0, 0, 0, 0, 7, // storage used
			0, 0, 0, 0, 0, 0, 0, 6, // storage index
			0, 0, 0, 0, 0, 0, 0, 5, // public key counts
			0, 0, 0, 0, 0, 0, 0, 3, // account id counter
		}

		// for v2->v3 migration, we are shrinking the public key counts from uint64 to uint32
		// so we need to decrease the storage used by 4 bytes
		decreaseInSize := uint64(4)

		migrated, err := environment.AccountStatusFromBytes(oldBytes)
		require.NoError(t, err)
		require.Equal(t, atree.SlabIndex{0, 0, 0, 0, 0, 0, 0, 6}, migrated.SlabIndex())
		require.Equal(t, uint32(5), migrated.PublicKeyCount())
		require.Equal(t, uint64(7)-decreaseInSize, migrated.StorageUsed())
		require.Equal(t, uint64(3), migrated.AccountIdCounter())
	})
}
