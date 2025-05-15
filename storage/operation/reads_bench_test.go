package operation_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
)

func BenchmarkRetrieve(t *testing.B) {
	dbtest.BenchWithStorages(t, func(t *testing.B, r storage.Reader, withWriter dbtest.WithWriter) {
		e := Entity{ID: 1337}
		require.NoError(t, withWriter(operation.Upsert(e.Key(), e)))

		t.ResetTimer()

		for i := 0; i < t.N; i++ {
			var readBack Entity
			require.NoError(t, operation.Retrieve(e.Key(), &readBack)(r))
		}
	})
}

func BenchmarkNonExist(t *testing.B) {
	dbtest.BenchWithStorages(t, func(t *testing.B, r storage.Reader, withWriter dbtest.WithWriter) {
		for i := 0; i < t.N; i++ {
			e := Entity{ID: uint64(i)}
			require.NoError(t, withWriter(operation.Upsert(e.Key(), e)))
		}

		t.ResetTimer()
		nonExist := Entity{ID: uint64(t.N + 1)}
		for i := 0; i < t.N; i++ {
			var exists bool
			require.NoError(t, operation.Exists(nonExist.Key(), &exists)(r))
		}
	})
}

func BenchmarkIterate(t *testing.B) {
	dbtest.BenchWithStorages(t, func(t *testing.B, r storage.Reader, withWriter dbtest.WithWriter) {
		prefix1 := []byte("prefix-1")
		prefix2 := []byte("prefix-2")
		for i := 0; i < t.N; i++ {
			e := Entity{ID: uint64(i)}
			key1 := append(prefix1, e.Key()...)
			key2 := append(prefix2, e.Key()...)

			require.NoError(t, withWriter(operation.Upsert(key1, e)))
			require.NoError(t, withWriter(operation.Upsert(key2, e)))
		}

		t.ResetTimer()
		var found [][]byte
		require.NoError(t, operation.Iterate(prefix1, prefix2, func(key []byte) error {
			found = append(found, key)
			return nil
		})(r), "should iterate forward without error")
	})
}
