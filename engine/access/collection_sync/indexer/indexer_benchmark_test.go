package indexer

import (
	"flag"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

var (
	collectionsPerHeight      = flag.Int("collections-per-height", 5, "Number of collections per height")
	transactionsPerCollection = flag.Int("transactions-per-collection", 2, "Number of transactions per collection")
)

// BenchmarkIndexCollectionsForBlock_Sequential benchmarks IndexCollectionsForBlock
// when called sequentially for multiple heights.
// It measures how many heights per second can be indexed.
//
// Benchmark results for 30s (Apple M1 Pro, Pebble storage, Nov 20, 2025):
//   - Default config (5 collections/height, 2 transactions/collection): ~197.9 heights/sec
//   - Higher load (10 collections/height, 5 transactions/collection): ~168.0 heights/sec
func BenchmarkIndexCollectionsForBlock_Sequential(b *testing.B) {
	unittest.RunWithPebbleDB(b, func(pdb *pebble.DB) {
		db := pebbleimpl.ToDB(pdb)
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()
		transactions := store.NewTransactions(metrics, db)
		collections := store.NewCollections(db, transactions)

		indexer := NewBlockCollectionIndexer(
			metrics,
			lockManager,
			db,
			collections,
		)

		// Pre-generate collections for all heights
		collectionsByHeight := make([][]*flow.Collection, b.N)
		for height := 0; height < b.N; height++ {
			cols := make([]*flow.Collection, *collectionsPerHeight)
			for i := 0; i < *collectionsPerHeight; i++ {
				col := unittest.CollectionFixture(*transactionsPerCollection)
				cols[i] = &col
			}
			collectionsByHeight[height] = cols
		}

		b.ResetTimer()
		b.ReportAllocs()

		for height := 0; height < b.N; height++ {
			err := indexer.IndexCollectionsForBlock(uint64(height), collectionsByHeight[height])
			require.NoError(b, err)
		}

		// Report heights per second
		b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "heights/sec")
	})
}

// BenchmarkIndexCollectionsForBlock_Concurrent benchmarks IndexCollectionsForBlock
// with 2 concurrent threads, each independently calling IndexCollectionsForBlock
// sequentially for the same height range. Both threads use the same data for each height.
// It measures how many heights per second each thread can process.
//
// Benchmark results for 30s (Apple M1 Pro, Pebble storage, Nov 20, 2025):
//   - Default config (5 collections/height, 2 transactions/collection):
//     Thread 1: ~185.9 heights/sec, Thread 2: ~185.9 heights/sec
func BenchmarkIndexCollectionsForBlock_Concurrent(b *testing.B) {
	unittest.RunWithPebbleDB(b, func(pdb *pebble.DB) {
		db := pebbleimpl.ToDB(pdb)
		lockManager := storage.NewTestingLockManager()
		metrics := metrics.NewNoopCollector()
		transactions := store.NewTransactions(metrics, db)
		collections := store.NewCollections(db, transactions)

		// Create separate indexer instances for each thread
		indexer := NewBlockCollectionIndexer(
			metrics,
			lockManager,
			db,
			collections,
		)

		// Pre-generate collections for all heights (shared between threads)
		collectionsByHeight := make([][]*flow.Collection, b.N)
		for height := 0; height < b.N; height++ {
			cols := make([]*flow.Collection, *collectionsPerHeight)
			for i := 0; i < *collectionsPerHeight; i++ {
				col := unittest.CollectionFixture(*transactionsPerCollection)
				cols[i] = &col
			}
			collectionsByHeight[height] = cols
		}

		b.ResetTimer()
		b.ReportAllocs()

		var wg sync.WaitGroup
		var thread1Elapsed, thread2Elapsed time.Duration

		// Thread 1: process heights sequentially
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			for height := 0; height < b.N; height++ {
				err := indexer.IndexCollectionsForBlock(uint64(height), collectionsByHeight[height])
				require.NoError(b, err)
			}
			thread1Elapsed = time.Since(start)
		}()

		// Thread 2: process the same heights sequentially
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			for height := 0; height < b.N; height++ {
				err := indexer.IndexCollectionsForBlock(uint64(height), collectionsByHeight[height])
				require.NoError(b, err)
			}
			thread2Elapsed = time.Since(start)
		}()

		wg.Wait()

		// Report heights per second for each thread
		thread1HeightsPerSec := float64(b.N) / thread1Elapsed.Seconds()
		thread2HeightsPerSec := float64(b.N) / thread2Elapsed.Seconds()
		b.ReportMetric(thread1HeightsPerSec, "heights/sec-thread1")
		b.ReportMetric(thread2HeightsPerSec, "heights/sec-thread2")
	})
}
