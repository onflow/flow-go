package delta_test

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/linxGnu/grocksdb"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// GENERAL COMMENT:
// running this test with
//
//	go test -bench=.  -benchmem
//
// will track the heap allocations for the Benchmarks
func BenchmarkStorage(b *testing.B) { benchmarkStorage(10000, b) } // 1_000_000

// register to read from previous batches
// insertion (bestcase vs worst case)
// high level of bootstrapped step - init point
// through set of steps (insert)

// BenchmarkStorage benchmarks the performance of the storage layer
func benchmarkStorage(steps int, b *testing.B) {
	// assumption: 1000 key updates per collection
	const (
		bootstrapSize      = 1_000_000_000 // 500_000_000
		numInsPerStep      = 1000
		keyNumberOfParts   = 2
		keyPartMinByteSize = 1
		keyPartMaxByteSize = 100
		valueMaxByteSize   = 1024
	)

	rand.Seed(time.Now().UnixNano())

	unittest.RunWithTempDir(b, func(dir string) {
		b.Logf("badger dir: %s", dir)
		// opts := badger.
		// 	DefaultOptions(dir).
		// 	WithKeepL0InMemory(true).

		// 	// the ValueLogFileSize option specifies how big the value of a
		// 	// key-value pair is allowed to be saved into badger.
		// 	// exceeding this limit, will fail with an error like this:
		// 	// could not store data: Value with size <xxxx> exceeded 1073741824 limit
		// 	// Maximum value size is 10G, needed by execution node
		// 	// TODO: finding a better max value for each node type
		// 	WithValueLogFileSize(128 << 23).
		// 	WithValueLogMaxEntries(100000) // Default is 1000000

		// db, err := badger.Open(opts)
		// require.NoError(b, err)

		// storage, err := delta.NewBadgerStore(db)
		// require.NoError(b, err)

		bbto := grocksdb.NewDefaultBlockBasedTableOptions()
		bbto.SetBlockCache(grocksdb.NewLRUCache(3 << 30))

		opts := grocksdb.NewDefaultOptions()
		opts.SetBlockBasedTableFactory(bbto)
		opts.SetCreateIfMissing(true)
		opts.SetMaxOpenFiles(8192)

		db, err := grocksdb.OpenDb(opts, dir)

		storage, err := delta.NewRocksStore(db, opts)
		require.NoError(b, err)

		// batchSize := 1000
		// steps := bootstrapSize / batchSize
		// for i := 0; i < steps; i++ {
		// 	owners := testutils.RandomValues(batchSize, keyPartMinByteSize, keyPartMaxByteSize)
		// 	keys := testutils.RandomValues(batchSize, keyPartMinByteSize, keyPartMaxByteSize)
		// 	values := testutils.RandomValues(batchSize, 1, valueMaxByteSize)
		// 	registers := make([]flow.RegisterEntry, batchSize)
		// 	for i := 0; i < batchSize; i++ {
		// 		registers[i] = flow.RegisterEntry{Key: flow.RegisterID{
		// 			Owner: string(owners[i]),
		// 			Key:   string(keys[i]),
		// 		},
		// 			Value: values[i]}
		// 	}

		// 	err = storage.Bootstrap(0, registers)
		// 	require.NoError(b, err)
		// }

		tempdir, err := os.MkdirTemp("", "flow-temp-data")
		require.NoError(b, err)

		err = storage.FastBootstrapWithRandomValues(tempdir, bootstrapSize, 32, 32, valueMaxByteSize)
		require.NoError(b, err)

		oracle, err := delta.NewOracle(storage)
		require.NoError(b, err)

		totalUpdateTimeNS := 0
		totalReadTimeNS := 0
		totalRegOperation := 0
		maxReadTimeNS := 0

		headers := []*flow.Header{
			unittest.BlockHeaderFixture(unittest.WithHeaderHeight(0)), // genesis
		}
		blockProductionIndex := 0
		blockSealedIndex := 1
		sealLatency := 10

		keysToRead := make([]ledger.Key, 0)

		for i := 0; i < steps; i++ {
			if i%1000 == 0 {
				b.Logf("progress: %d percent", i/steps*100)
			}
			// send seal info
			if blockSealedIndex >= blockProductionIndex+sealLatency {
				h := headers[blockSealedIndex]
				oracle.BlockIsSealed(h.ID(), h)
				require.NoError(b, err)
				require.Equal(b, oracle.BlocksInFlight(), sealLatency)

				blockSealedIndex++
			}

			// construct new block

			parentHeader := headers[blockProductionIndex]
			newHeader := unittest.BlockHeaderWithParentFixture(parentHeader)
			headers = append(headers, newHeader)
			blockProductionIndex++

			view, err := oracle.NewBlockView(newHeader.ID(), newHeader)
			require.NoError(b, err)

			keys := testutils.RandomUniqueKeys(numInsPerStep, keyNumberOfParts, keyPartMinByteSize, keyPartMaxByteSize)
			values := testutils.RandomValues(numInsPerStep, 1, valueMaxByteSize)

			totalRegOperation += len(keys)
			start := time.Now()

			for i, key := range keys {
				err := view.Set(string(key.KeyParts[0].Value), string(key.KeyParts[1].Value), values[i])
				require.NoError(b, err)
			}

			elapsed := time.Since(start)
			totalUpdateTimeNS += int(elapsed.Nanoseconds())

			// append the first 10 keys for future reads
			keysToRead = append(keysToRead, keys[:10]...)

			// read values and compare values
			for _, key := range keys {
				start = time.Now()
				v, err := view.Get(string(key.KeyParts[0].Value), string(key.KeyParts[1].Value))
				require.NoError(b, err)
				require.True(b, len(v) > 0)
				elapsed = time.Since(start)
				elapsedns := int(elapsed.Nanoseconds())
				if elapsedns > maxReadTimeNS {
					maxReadTimeNS = elapsedns
				}
				totalReadTimeNS += elapsedns
			}
		}

		// read special key
		key := "random key"
		start := time.Now()
		_, _, err = storage.UnsafeRead(key)
		fmt.Println(">>>>>", time.Since(start))
		require.NoError(b, err)

		b.ReportMetric(float64(totalUpdateTimeNS/steps), "update_time_(ns)")
		b.ReportMetric(float64(totalUpdateTimeNS/totalRegOperation), "update_time_per_reg_(ns)")

		b.ReportMetric(float64(totalReadTimeNS/steps), "read_time_(ns)")
		b.ReportMetric(float64(totalReadTimeNS/totalRegOperation), "read_time_per_reg_(ns)")
		b.ReportMetric(float64(maxReadTimeNS), "max_read_time(ns)")

	})
}
