package sync_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/sync"
	"github.com/onflow/flow-go/model/flow"
)

func TestInMemoryStorageProvider(t *testing.T) {
	sp := sync.NewInMemoryStorageProvider(sync.EmptySnapshot)

	// at start
	_, err := sp.GetSnapshotAt(0)
	require.NoError(t, err)

	_, err = sp.GetSnapshotAt(1)
	require.NoError(t, err)

	_, err = sp.GetSnapshotAt(2)
	require.Error(t, err)

	// add values
	owner := []byte("owner")
	key := []byte("key")
	deltaKey := flow.RegisterID{
		string(owner), string(key),
	}
	// block additions
	for i := 1; i < 11; i++ {
		_, err := sp.GetSnapshotAt(uint64(i))
		require.NoError(t, err)
		delta := map[flow.RegisterID]flow.RegisterValue{
			deltaKey: []byte{byte(i)},
		}
		sp.OnBlockExecuted(delta)
	}

	for i := 1; i < 11; i++ {
		s, err := sp.GetSnapshotAt(uint64(i))
		require.NoError(t, err)

		ret, err := s.GetValue(owner, key)
		require.NoError(t, err)

		require.Equal(t, []byte{byte(i)}, ret)
	}
}
