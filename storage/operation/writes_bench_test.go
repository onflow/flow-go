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

func BenchmarkUpsert(t *testing.B) {
	dbtest.BenchWithStorages(t, func(t *testing.B, r storage.Reader, withWriter dbtest.WithWriter) {
		for i := 0; i < t.N; i++ {
			e := Entity{ID: uint64(i)}
			require.NoError(t, withWriter(operation.Upsert(e.Key(), e)))
		}
	})
}

func BenchmarkRemove(t *testing.B) {
	dbtest.BenchWithStorages(t, func(t *testing.B, r storage.Reader, withWriter dbtest.WithWriter) {
		n := t.N
		for i := 0; i < n; i++ {
			e := Entity{ID: uint64(i)}
			require.NoError(t, withWriter(operation.Upsert(e.Key(), e)))
		}
		t.ResetTimer()
		for i := 0; i < n; i++ {
			e := Entity{ID: uint64(i)}
			require.NoError(t, withWriter(operation.Remove(e.Key())))
		}
	})
}

func BenchmarkRemoveByPrefix(t *testing.B) {
	dbtest.BenchWithStorages(t, func(t *testing.B, r storage.Reader, withWriter dbtest.WithWriter) {
		prefix := []byte("prefix")
		for i := 0; i < t.N; i++ {
			e := Entity{ID: uint64(i)}
			key := append(prefix, e.Key()...)
			require.NoError(t, withWriter(operation.Upsert(key, e)))
		}
		t.ResetTimer()
		require.NoError(t, withWriter(operation.RemoveByPrefix(r, prefix)))
	})
}
