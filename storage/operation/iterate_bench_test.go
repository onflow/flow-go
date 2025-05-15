package operation_test

import (
	"fmt"
	"testing"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/merr"

	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack"
)

func BenchmarkFindHighestAtOrBelowByPrefixUsingIterator(t *testing.B) {
	dbtest.BenchWithStorages(t, func(t *testing.B, r storage.Reader, withWriter dbtest.WithWriter) {
		const count = 500
		const step = 2

		prefix := byte(67)
		for i := range count {
			k := operation.MakePrefix(prefix, uint64(i)*step)
			e := Entity{ID: uint64(i * 2)}
			require.NoError(t, withWriter(operation.Upsert(k, e)))
		}

		t.ResetTimer()

		height := uint64(500)
		for i := 0; i < t.N; i++ {
			var entity Entity
			require.NoError(t, findHighestAtOrBelowByPrefixUsingIterator(r, []byte{prefix}, height, &entity))
		}
	})
}

func BenchmarkFindHighestAtOrBelowByPrefixUsingSeeker(t *testing.B) {
	dbtest.BenchWithStorages(t, func(t *testing.B, r storage.Reader, withWriter dbtest.WithWriter) {
		const count = 500
		const step = 2

		prefix := byte(67)
		for i := range count {
			k := operation.MakePrefix(prefix, uint64(i)*step)
			e := Entity{ID: uint64(i * 2)}
			require.NoError(t, withWriter(operation.Upsert(k, e)))
		}

		t.ResetTimer()

		height := uint64(500)
		for i := 0; i < t.N; i++ {
			var entity Entity
			require.NoError(t, findHighestAtOrBelowByPrefixUsingSeeker(r, []byte{prefix}, height, &entity))
		}
	})
}

// findHighestAtOrBelowByPrefixUsingIterator is the original operation.FindHighestAtOrBelowByPrefix().
func findHighestAtOrBelowByPrefixUsingIterator(r storage.Reader, prefix []byte, height uint64, entity interface{}) (errToReturn error) {
	if len(prefix) == 0 {
		return fmt.Errorf("prefix must not be empty")
	}

	key := append(prefix, operation.EncodeKeyPart(height)...)
	it, err := r.NewIter(prefix, key, storage.DefaultIteratorOptions())
	if err != nil {
		return fmt.Errorf("can not create iterator: %w", err)
	}
	defer func() {
		errToReturn = merr.CloseAndMergeError(it, errToReturn)
	}()

	var highestKey []byte

	// find highest value below the given height
	for it.First(); it.Valid(); it.Next() {
		// copy the key to avoid the underlying slices of the key
		// being modified by the Next() call
		highestKey = it.IterItem().KeyCopy(highestKey)
	}

	if len(highestKey) == 0 {
		return storage.ErrNotFound
	}

	// read the value of the highest key
	val, closer, err := r.Get(highestKey)
	if err != nil {
		return err
	}

	defer func() {
		errToReturn = merr.CloseAndMergeError(closer, errToReturn)
	}()

	err = msgpack.Unmarshal(val, entity)
	if err != nil {
		return irrecoverable.NewExceptionf("failed to decode value: %w", err)
	}

	return nil
}

func findHighestAtOrBelowByPrefixUsingSeeker(r storage.Reader, prefix []byte, height uint64, entity interface{}) (errToReturn error) {
	return operation.FindHighestAtOrBelowByPrefix(r, prefix, height, entity)
}
