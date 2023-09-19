package pebble

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"path"
	"strconv"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/pebble/registers"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestRegisters_Initialize
func TestRegisters_Initialize(t *testing.T) {
	t.Parallel()
	p, dir := unittest.TempPebbleDBWithOpts(t, nil)
	// fail on blank database without FirstHeight and LastHeight set
	_, err := NewRegisters(p)
	require.Error(t, err)
	err = os.RemoveAll(dir)
	require.NoError(t, err)
}

// TestRegisters_Get tests the expected Get function behavior on a single height
func TestRegisters_Get(t *testing.T) {
	t.Parallel()
	height1 := uint64(1)
	RunWithRegistersStorageAtHeight1(t, func(r *Registers) {
		// invalid keys return correct error type
		invalidKey := flow.RegisterID{Owner: "invalid", Key: "invalid"}
		_, err := r.Get(height1, invalidKey)
		require.ErrorIs(t, err, storage.ErrNotFound)

		// insert new data
		height2 := uint64(2)
		key1 := flow.RegisterID{Owner: "owner", Key: "key1"}
		expectedValue1 := []byte("value1")
		entries := flow.RegisterEntries{
			{Key: key1, Value: expectedValue1},
		}

		err = r.Store(height2, entries)
		require.NoError(t, err)

		// happy path
		value1, err := r.Get(height2, key1)
		require.NoError(t, err)
		require.Equal(t, expectedValue1, value1)

		// out of range
		beforeFirstHeight := uint64(0)
		_, err = r.Get(beforeFirstHeight, key1)
		require.ErrorIs(t, err, storage.ErrHeightNotIndexed)
		afterLatestHeight := uint64(3)
		_, err = r.Get(afterLatestHeight, key1)
		require.ErrorIs(t, err, storage.ErrHeightNotIndexed)
	})
}

// TestRegisters_Store tests the expected store behaviour on a single height
func TestRegisters_Store(t *testing.T) {
	t.Parallel()
	RunWithRegistersStorageAtHeight1(t, func(r *Registers) {
		// insert new data
		key1 := flow.RegisterID{Owner: "owner", Key: "key1"}
		expectedValue1 := []byte("value1")
		entries := flow.RegisterEntries{
			{Key: key1, Value: expectedValue1},
		}
		height2 := uint64(2)
		err := r.Store(height2, entries)
		require.NoError(t, err)

		// idempotent at same height
		err = r.Store(height2, entries)
		require.NoError(t, err)

		// out of range
		height5 := uint64(5)
		err = r.Store(height5, entries)
		require.Error(t, err)

		height1 := uint64(1)
		err = r.Store(height1, entries)
		require.Error(t, err)

	})
}

// TestRegisters_Store_RoundTrip tests the round trip of a payload storage.
func TestRegisters_Store_RoundTrip(t *testing.T) {
	t.Parallel()
	minHeight := uint64(2)
	RunWithRegistersStorageAtInitialHeights(t, minHeight, minHeight, func(r *Registers) {
		key1 := flow.RegisterID{Owner: "owner", Key: "key1"}
		expectedValue1 := []byte("value1")
		entries := flow.RegisterEntries{
			{Key: key1, Value: expectedValue1},
		}
		testHeight := minHeight + 1
		// happy path
		err := r.Store(testHeight, entries)
		require.NoError(t, err)

		// lookup with exact height returns the correct value
		value1, err := r.Get(testHeight, key1)
		require.NoError(t, err)
		require.Equal(t, expectedValue1, value1)

		value11, err := r.Get(testHeight, key1)
		require.NoError(t, err)
		require.Equal(t, expectedValue1, value11)
	})
}

// TestRegisters_Store_Versioning tests the scan functionality for the most recent value
func TestRegisters_Store_Versioning(t *testing.T) {
	t.Parallel()
	RunWithRegistersStorageAtHeight1(t, func(r *Registers) {
		// Save key11 is a prefix of the key1, and we save it first.
		// It should be invisible for our prefix scan.
		key11 := flow.RegisterID{Owner: "owner", Key: "key11"}
		expectedValue11 := []byte("value11")

		key1 := flow.RegisterID{Owner: "owner", Key: "key1"}
		expectedValue1 := []byte("value1")
		entries1 := flow.RegisterEntries{
			{Key: key1, Value: expectedValue1},
			{Key: key11, Value: expectedValue11},
		}

		height2 := uint64(2)

		// check increment in height after Store()
		err := r.Store(height2, entries1)
		require.NoError(t, err)

		// Add new version of key1.
		height3 := uint64(3)
		expectedValue1ge3 := []byte("value1ge3")
		entries3 := flow.RegisterEntries{
			{Key: key1, Value: expectedValue1ge3},
		}

		// check increment in height after Store()
		err = r.Store(height3, entries3)
		require.NoError(t, err)
		updatedHeight, err := r.LatestHeight()
		require.NoError(t, err)
		require.Equal(t, updatedHeight, height3)

		// test old version at previous height
		value1, err := r.Get(height2, key1)
		require.NoError(t, err)
		require.Equal(t, expectedValue1, value1)

		// test new version at new height
		value1, err = r.Get(height3, key1)
		require.NoError(t, err)
		require.Equal(t, expectedValue1ge3, value1)

		// test unchanged key at incremented height
		value11, err := r.Get(height3, key11)
		require.NoError(t, err)
		require.Equal(t, expectedValue11, value11)
	})
}

// Benchmark_PayloadStorage benchmarks the SetBatch method.
func Benchmark_PayloadStorage(b *testing.B) {
	cache := pebble.NewCache(32 << 20)
	defer cache.Unref()
	opts := DefaultPebbleOptions(cache, registers.NewMVCCComparer())

	dbpath := path.Join(b.TempDir(), "benchmark1.db")
	db, err := pebble.Open(dbpath, opts)
	require.NoError(b, err)
	s, err := NewRegisters(db)
	require.NoError(b, err)
	require.NotNil(b, s)

	batchSizeKey := flow.NewRegisterID("batch", "size")
	const maxBatchSize = 1024
	var totalBatchSize int

	keyForBatchSize := func(i int) flow.RegisterID {
		return flow.NewRegisterID("batch", strconv.Itoa(i))
	}
	valueForHeightAndKey := func(i, j int) []byte {
		return []byte(fmt.Sprintf("%d-%d", i, j))
	}
	b.ResetTimer()

	// Write a random number of entries in each batch.
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		batchSize := rand.Intn(maxBatchSize) + 1
		totalBatchSize += batchSize
		entries := make(flow.RegisterEntries, 1, batchSize)
		entries[0] = flow.RegisterEntry{
			Key:   batchSizeKey,
			Value: []byte(fmt.Sprintf("%d", batchSize)),
		}
		for j := 1; j < batchSize; j++ {
			entries = append(entries, flow.RegisterEntry{
				Key:   keyForBatchSize(j),
				Value: valueForHeightAndKey(i, j),
			})
		}
		b.StartTimer()

		err = s.Store(uint64(i), entries)
		require.NoError(b, err)
	}

	b.StopTimer()

	// verify written batches
	for i := 0; i < b.N; i++ {
		// get number of batches written for height
		batchSizeBytes, err := s.Get(uint64(i), batchSizeKey)
		require.NoError(b, err)
		batchSize, err := strconv.Atoi(string(batchSizeBytes))
		require.NoError(b, err)

		// verify that all entries can be read with correct values
		for j := 1; j < batchSize; j++ {
			value, err := s.Get(uint64(i), keyForBatchSize(j))
			require.NoError(b, err)
			require.Equal(b, valueForHeightAndKey(i, j), value)
		}

		// verify that the rest of the batches either do not exist or have a previous height
		for j := batchSize; j < maxBatchSize+1; j++ {
			value, err := s.Get(uint64(i), keyForBatchSize(j))
			require.Nil(b, err)

			if len(value) > 0 {
				ij := bytes.Split(value, []byte("-"))

				// verify that we've got a value for a previous height
				height, err := strconv.Atoi(string(ij[0]))
				require.NoError(b, err)
				require.Lessf(b, height, i, "height: %d, j: %d", height, j)

				// verify that we've got a value corresponding to the index
				index, err := strconv.Atoi(string(ij[1]))
				require.NoError(b, err)
				require.Equal(b, index, j)
			}
		}
	}
}

func RunWithRegistersStorageAtInitialHeights(tb testing.TB, first uint64, latest uint64, f func(r *Registers)) {
	cache := pebble.NewCache(1 << 20)
	opts := DefaultPebbleOptions(cache, registers.NewMVCCComparer())
	unittest.RunWithConfiguredPebbleInstance(tb, opts, func(p *pebble.DB) {
		// insert initial heights to pebble
		require.NoError(tb, p.Set(FirstHeightKey(), EncodedUint64(first), nil))
		require.NoError(tb, p.Set(LatestHeightKey(), EncodedUint64(latest), nil))
		r, err := NewRegisters(p)
		require.NoError(tb, err)
		f(r)
	})
}

func RunWithRegistersStorageAtHeight1(tb testing.TB, f func(r *Registers)) {
	defaultHeight := uint64(1)
	RunWithRegistersStorageAtInitialHeights(tb, defaultHeight, defaultHeight, f)
}
