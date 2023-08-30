package payload

import (
	"bytes"
	"fmt"
	"math/rand"
	"path"
	"strconv"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/require"
)

// Test_PayloadStorage_RoundTrip tests the round trip of a payload storage.
func Test_PayloadStorage_RoundTrip(t *testing.T) {
	t.Parallel()

	cache := pebble.NewCache(1 << 20)
	defer cache.Unref()

	dbpath := path.Join(t.TempDir(), "roundtrip.db")
	s, err := NewStorage(dbpath, cache)
	require.NoError(t, err)
	require.NotNil(t, s)

	key1 := flow.RegisterID{Owner: "owner", Key: "key1"}
	expectedValue1 := []byte("value1")
	entries := flow.RegisterEntries{
		{Key: key1, Value: expectedValue1},
	}

	minHeight := uint64(2)
	err = s.BatchSetPayload(minHeight, entries)
	require.NoError(t, err)

	value1, err := s.GetPayload(minHeight, key1)
	require.NoError(t, err)
	require.Equal(t, expectedValue1, value1)

	value1, err = s.GetPayload(minHeight+1, key1)
	require.NoError(t, err)
	require.Equal(t, expectedValue1, value1)

	value1, err = s.GetPayload(minHeight-1, key1)
	require.Nil(t, err)
	require.Empty(t, value1)

	err = s.Close()
	require.NoError(t, err)
}

func Test_PayloadStorage_Versioning(t *testing.T) {
	t.Parallel()

	cache := pebble.NewCache(1 << 20)
	defer cache.Unref()

	dbpath := path.Join(t.TempDir(), "versionning.db")
	s, err := NewStorage(dbpath, cache)
	require.NoError(t, err)
	require.NotNil(t, s)

	// Save key11 is a prefix of the key1 and we save it first.
	// It should be invisible for our prefix scan.
	key11 := flow.RegisterID{Owner: "owner", Key: "key11"}
	expectedValue11 := []byte("value11")

	key1 := flow.RegisterID{Owner: "owner", Key: "key1"}
	expectedValue1 := []byte("value1")
	entries1 := flow.RegisterEntries{
		{Key: key1, Value: expectedValue1},
		{Key: key11, Value: expectedValue11},
	}

	height1 := uint64(1)
	err = s.BatchSetPayload(height1, entries1)
	require.NoError(t, err)

	// Test non-existent prefix.
	key := flow.RegisterID{Owner: "owner", Key: "key"}
	value0, err := s.GetPayload(height1, key)
	require.Nil(t, err)
	require.Empty(t, value0)

	// Add new version of key1.
	height3 := uint64(3)
	expectedValue1ge3 := []byte("value1ge3")
	entries3 := flow.RegisterEntries{
		{Key: key1, Value: expectedValue1ge3},
	}
	err = s.BatchSetPayload(height3, entries3)
	require.NoError(t, err)

	value1, err := s.GetPayload(height1, key1)
	require.NoError(t, err)
	require.Equal(t, expectedValue1, value1)

	value1, err = s.GetPayload(height3-1, key1)
	require.NoError(t, err)
	require.Equal(t, expectedValue1, value1)

	// test new version
	value1, err = s.GetPayload(height3, key1)
	require.NoError(t, err)
	require.Equal(t, expectedValue1ge3, value1)

	value1, err = s.GetPayload(height3+1, key1)
	require.NoError(t, err)
	require.Equal(t, expectedValue1ge3, value1)

	err = s.Close()
	require.NoError(t, err)
}

// Benchmark_PayloadStorage benchmarks the SetBatch method.
func Benchmark_PayloadStorage(b *testing.B) {
	cache := pebble.NewCache(32 << 20)
	defer cache.Unref()

	dbpath := path.Join(b.TempDir(), "benchmark1.db")
	s, err := NewStorage(dbpath, cache)
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

		err = s.BatchSetPayload(uint64(i), entries)
		require.NoError(b, err)
	}

	b.StopTimer()

	// verify written batches
	for i := 0; i < b.N; i++ {
		// get number of batches written for height
		batchSizeBytes, err := s.GetPayload(uint64(i), batchSizeKey)
		require.NoError(b, err)
		batchSize, err := strconv.Atoi(string(batchSizeBytes))
		require.NoError(b, err)

		// verify that all entries can be read with correct values
		for j := 1; j < batchSize; j++ {
			value, err := s.GetPayload(uint64(i), keyForBatchSize(j))
			require.NoError(b, err)
			require.Equal(b, valueForHeightAndKey(i, j), value)
		}

		// verify that the rest of the batches either do not exist or have a previous height
		for j := batchSize; j < maxBatchSize+1; j++ {
			value, err := s.GetPayload(uint64(i), keyForBatchSize(j))
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

// TODO(rbtz): add parallel benchmarks
