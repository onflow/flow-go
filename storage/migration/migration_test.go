package migration

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestGeneratePrefixes(t *testing.T) {
	t.Run("OneBytePrefix", func(t *testing.T) {
		prefixes := GeneratePrefixes(1)
		require.Len(t, prefixes, 256)
		require.Equal(t, []byte{0x00}, prefixes[0])
		require.Equal(t, []byte{0x01}, prefixes[1])
		require.Equal(t, []byte{0xfe}, prefixes[254])
		require.Equal(t, []byte{0xff}, prefixes[255])
	})

	t.Run("TwoBytePrefix", func(t *testing.T) {
		prefixes := GeneratePrefixes(2)
		require.Len(t, prefixes, 65536)
		require.Equal(t, []byte{0x00, 0x00}, prefixes[0])
		require.Equal(t, []byte{0x00, 0x01}, prefixes[1])
		require.Equal(t, []byte{0xff, 0xfe}, prefixes[65534])
		require.Equal(t, []byte{0xff, 0xff}, prefixes[65535])
	})
}

func runMigrationTestCase(t *testing.T, testData map[string]string, cfg MigrationConfig) {
	unittest.RunWithBadgerDBAndPebbleDB(t, func(badgerDB *badger.DB, pebbleDB *pebble.DB) {
		// Load Badger with test data
		require.NoError(t, badgerDB.Update(func(txn *badger.Txn) error {
			for k, v := range testData {
				if err := txn.Set([]byte(k), []byte(v)); err != nil {
					return err
				}
			}
			return nil
		}))

		// Run migration
		err := CopyFromBadgerToPebbleSSTables(badgerDB, pebbleDB, cfg)
		require.NoError(t, err)

		// Validate each key
		for k, expected := range testData {
			val, closer, err := pebbleDB.Get([]byte(k))
			require.NoError(t, err, "pebbleDB.Get failed for key %s", k)
			require.Equal(t, expected, string(val), "mismatched value for key %s", k)
			require.NoError(t, closer.Close())
		}

		// Validate: Ensure Pebble have no additional key
		iter, err := pebbleDB.NewIter(nil)
		require.NoError(t, err)
		defer iter.Close()

		seen := make(map[string]string)

		for iter.First(); iter.Valid(); iter.Next() {
			k := string(iter.Key())
			v := string(iter.Value())

			expectedVal, ok := testData[k]
			require.True(t, ok, "unexpected key found in PebbleDB: %s", k)
			require.Equal(t, expectedVal, v, "mismatched value for key %s", k)

			seen[k] = v
		}
		require.NoError(t, iter.Error(), "error iterating over PebbleDB")

		// Ensure all expected keys were seen
		require.Equal(t, len(testData), len(seen), "PebbleDB key count mismatch")
	})
}

// Simple deterministic dataset
func TestMigrationWithSimpleData(t *testing.T) {
	data := map[string]string{
		"a":      "a single key byte",
		"z":      "a single key byte",
		"apple":  "fruit",
		"banana": "yellow",
		"carrot": "vegetable",
		"dog":    "animal",
		"egg":    "protein",
	}
	cfg := MigrationConfig{
		BatchByteSize:          1024,
		ReaderWorkerCount:      2,
		WriterWorkerCount:      2,
		ReaderShardPrefixBytes: 1,
	}
	runMigrationTestCase(t, data, cfg)
}

// Simple deterministic dataset
func TestMigrationWithSimpleDataAnd2PrefixBytes(t *testing.T) {
	data := map[string]string{
		"a":      "a single key byte",
		"z":      "a single key byte",
		"apple":  "fruit",
		"banana": "yellow",
		"carrot": "vegetable",
		"dog":    "animal",
		"egg":    "protein",
	}
	cfg := MigrationConfig{
		BatchByteSize:          1024,
		ReaderWorkerCount:      2,
		WriterWorkerCount:      2,
		ReaderShardPrefixBytes: 2,
	}
	runMigrationTestCase(t, data, cfg)
}

// Randomized data to simulate fuzzing
func TestMigrationWithFuzzyData(t *testing.T) {
	data := generateRandomKVData(500, 10, 50)
	cfg := MigrationConfig{
		BatchByteSize:          2048,
		ReaderWorkerCount:      4,
		WriterWorkerCount:      2,
		ReaderShardPrefixBytes: 1,
	}
	runMigrationTestCase(t, data, cfg)
}

// Fuzzy data with 2-byte prefix shard config
func TestMigrationWithFuzzyDataAndPrefix2(t *testing.T) {
	data := generateRandomKVData(500, 10, 50)
	cfg := MigrationConfig{
		BatchByteSize:          2048,
		ReaderWorkerCount:      8,
		WriterWorkerCount:      4,
		ReaderShardPrefixBytes: 2,
	}
	runMigrationTestCase(t, data, cfg)
}

// Utility: Generate random key-value pairs
func generateRandomKVData(count, keyLen, valLen int) map[string]string {
	rng := rand.New(rand.NewSource(42)) // deterministic
	data := make(map[string]string, count)
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	randomStr := func(n int) string {
		b := make([]rune, n)
		for i := range b {
			b[i] = letters[rng.Intn(len(letters))]
		}
		return string(b)
	}

	for i := 0; i < count; i++ {
		k := randomStr(keyLen)
		v := randomStr(valLen)
		data[k] = v
	}
	return data
}

func BenchmarkCopyFromBadgerToPebble(b *testing.B) {
	// Configuration
	const (
		numEntries        = 1_000_000
		keySize           = 16
		valueSize         = 128
		batchByteSize     = 4 * 1024 * 1024 // 4MB
		readerWorkerCount = 4
		writerWorkerCount = 4
		prefixBytes       = 1
	)

	// Setup: Create temp dirs for Badger and Pebble
	unittest.RunWithTempDirs(b, func(badgerDir, pebbleDir string) {
		// Open Badger
		badgerOpts := badger.DefaultOptions(badgerDir).WithLogger(nil)
		badgerDB, err := badger.Open(badgerOpts)
		if err != nil {
			b.Fatalf("failed to open BadgerDB: %v", err)
		}
		defer badgerDB.Close()

		// Insert random data into Badger
		rng := rand.New(rand.NewSource(42))
		batchSize := 100
		batchCount := numEntries / batchSize
		for range batchSize {
			err = badgerDB.Update(func(txn *badger.Txn) error {
				for range batchCount {
					key := make([]byte, keySize)
					value := make([]byte, valueSize)
					rng.Read(key)
					rng.Read(value)

					if err := txn.Set(key, value); err != nil {
						return fmt.Errorf("failed to set key %x: %w", key, err)
					}
				}
				return nil
			})
			if err != nil {
				b.Fatalf("failed to insert data into BadgerDB: %v", err)
			}
		}

		// Open Pebble
		pebbleDB, err := pebble.Open(pebbleDir, &pebble.Options{})
		if err != nil {
			b.Fatalf("failed to open PebbleDB: %v", err)
		}
		defer pebbleDB.Close()

		// Setup migration config
		cfg := MigrationConfig{
			BatchByteSize:          batchByteSize,
			ReaderWorkerCount:      readerWorkerCount,
			WriterWorkerCount:      writerWorkerCount,
			ReaderShardPrefixBytes: prefixBytes,
		}

		// Benchmark the migration
		b.ResetTimer()
		b.StartTimer()
		if err := CopyFromBadgerToPebble(badgerDB, pebbleDB, cfg); err != nil {
			b.Fatalf("migration failed: %v", err)
		}
		b.StopTimer()
	})
}
