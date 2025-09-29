package store

import (
	"bytes"
	"crypto/rand"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestGroupCacheRemoveGroups(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		store := func(rw storage.ReaderBatchWriter, key TwoIdentifier, val []byte) error {
			return operation.UpsertByKey(rw.Writer(), key[:], val)
		}

		retrieve := func(r storage.Reader, key TwoIdentifier) ([]byte, error) {
			var val []byte
			err := operation.RetrieveByKey(r, key[:], &val)
			if err != nil {
				return nil, err
			}
			return val, nil
		}

		cache := mustNewGroupCache(
			t,
			metrics.NewNoopCollector(),
			"test",
			FirstIDFromTwoIdentifier, // cache records are grouped by first identifier of keys
			withStore(store),
			withRetrieve(retrieve),
		)

		const groupCount = 5
		const itemCountPerGroup = 5

		groupIDs := make([]flow.Identifier, 0, groupCount)
		keyValuePairs := make(map[TwoIdentifier][]byte)
		for range groupCount {
			groupID := unittest.IdentifierFixture()

			groupIDs = append(groupIDs, groupID)

			for range itemCountPerGroup {
				var itemID TwoIdentifier
				n := copy(itemID[:], groupID[:])
				_, _ = rand.Read(itemID[n:])

				val := unittest.RandomBytes(128)

				keyValuePairs[itemID] = val
			}
		}

		// Store items in DB and cache
		require.NoError(t, db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			for key, val := range keyValuePairs {
				err := cache.PutTx(rw, key, val)
				if err != nil {
					return err
				}
			}
			return nil
		}))

		// Retrieve stored item from cache
		for key, val := range keyValuePairs {
			cached, err := cache.Get(db.Reader(), key)
			require.NoError(t, err)
			require.Equal(t, val, cached)
		}

		// Remove items in the first group in DB.
		groupIDToRemove := groupIDs[0]
		groupIDs = groupIDs[1:]

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.RemoveByKeyPrefix(rw.GlobalReader(), rw.Writer(), groupIDToRemove[:])
		})
		require.NoError(t, err)

		// Remove cached items in the first group
		removedCount := cache.RemoveGroup(groupIDToRemove)
		require.Equal(t, itemCountPerGroup, removedCount)

		// Retrieve removed and stored items
		for key, val := range keyValuePairs {
			isRemovedItem := bytes.Equal(key[:flow.IdentifierLen], groupIDToRemove[:])

			cached, err := cache.Get(db.Reader(), key)
			if isRemovedItem {
				require.ErrorIs(t, err, storage.ErrNotFound)
			} else {
				require.NoError(t, err)
				require.Equal(t, val, cached)
			}
		}

		// Remove group that is already removed
		removedCount = cache.RemoveGroup(groupIDToRemove)
		require.Equal(t, 0, removedCount)

		// Remove all groups in DB except the last goup.
		groupIDsToRemove := groupIDs[:len(groupIDs)-1]
		groupIDs = groupIDs[len(groupIDs)-1:]

		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			for _, groupID := range groupIDsToRemove {
				err := operation.RemoveByKeyPrefix(rw.GlobalReader(), rw.Writer(), groupID[:])
				if err != nil {
					return err
				}
			}
			return err
		})
		require.NoError(t, err)

		// Remove all groups in cache except the last goup.
		removedCount = cache.RemoveGroups(groupIDsToRemove)
		require.Equal(t, itemCountPerGroup*len(groupIDsToRemove), removedCount)

		// Retrieve removed and stored items
		for key, val := range keyValuePairs {
			isRemovedItem := !slices.Contains(groupIDs, flow.Identifier(key[:flow.IdentifierLen]))

			cached, err := cache.Get(db.Reader(), key)
			if isRemovedItem {
				require.ErrorIs(t, err, storage.ErrNotFound)
			} else {
				require.NoError(t, err)
				require.Equal(t, val, cached)
			}
		}
	})
}

func BenchmarkCacheRemoveGroup(b *testing.B) {
	const txCountPerBlock = 5

	benchmarks := []struct {
		name        string
		cacheSize   int
		removeCount int
	}{
		{name: "cache size 1,000, remove count 25", cacheSize: 1_000, removeCount: 25},
		{name: "cache size 2,000, remove count 25", cacheSize: 2_000, removeCount: 25},
		{name: "cache size 3,000, remove count 25", cacheSize: 3_000, removeCount: 25},
		{name: "cache size 4,000, remove count 25", cacheSize: 4_000, removeCount: 25},
		{name: "cache size 5,000, remove count 25", cacheSize: 5_000, removeCount: 25},
		{name: "cache size 6,000, remove count 25", cacheSize: 6_000, removeCount: 25},
		{name: "cache size 7,000, remove count 25", cacheSize: 7_000, removeCount: 25},
		{name: "cache size 8,000, remove count 25", cacheSize: 8_000, removeCount: 25},
		{name: "cache size 9,000, remove count 25", cacheSize: 9_000, removeCount: 25},
		{name: "cache size 10,000, remove count 25", cacheSize: 10_000, removeCount: 25},
		{name: "cache size 20,000, remove count 25", cacheSize: 20_000, removeCount: 25},
		{name: "cache size 10,000, remove count 5,000", cacheSize: 10_000, removeCount: 5_000},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			blockCount := bm.cacheSize/txCountPerBlock + 1

			blockIDs := make([]flow.Identifier, blockCount)
			for i := range len(blockIDs) {
				blockIDs[i] = unittest.IdentifierFixture()
			}

			txIDs := make([]flow.Identifier, blockCount*txCountPerBlock)
			for i := range len(txIDs) {
				txIDs[i] = unittest.IdentifierFixture()
			}

			prefixCount := bm.removeCount / txCountPerBlock
			var removePrefixes []flow.Identifier
			for blockIDIndex := len(blockIDs) - 1; len(removePrefixes) < prefixCount; blockIDIndex-- {
				blockID := blockIDs[blockIDIndex]
				removePrefixes = append(removePrefixes, blockID)
			}

			b.ResetTimer()

			for range b.N {
				b.StopTimer()

				cache := mustNewGroupCache(
					b,
					metrics.NewNoopCollector(),
					metrics.ResourceTransactionResults,
					func(key TwoIdentifier) flow.Identifier { return flow.Identifier(key[:flow.IdentifierLen]) },
					withLimit[TwoIdentifier, struct{}](uint(bm.cacheSize)),
					withStore(noopStore[TwoIdentifier, struct{}]),
					withRetrieve(noRetrieve[TwoIdentifier, struct{}]),
				)

				for i, blockID := range blockIDs {
					for _, txID := range txIDs[i*txCountPerBlock : (i+1)*txCountPerBlock] {
						var key TwoIdentifier
						n := copy(key[:], blockID[:])
						copy(key[n:], txID[:])
						cache.Insert(key, struct{}{})
					}
				}

				b.StartTimer()

				cache.RemoveGroups(removePrefixes)
			}
		})
	}
}
