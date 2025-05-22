package migration

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/module/util"
)

type MigrationConfig struct {
	BatchByteSize     int // the size of each batch to write to pebble
	ReaderWorkerCount int // the number of workers to read from badger
	WriterWorkerCount int // the number of workers to write to the pebble

	// number of prefix bytes used to assign iterator workload
	// e.g, if the number is 1, it means the first byte of the key is used to divide into 256 key space,
	// and each worker will be assigned to iterate all keys with the same first byte.
	// Since keys are not evenly distributed, especially some table under a certain prefix byte may have
	// a lot more data than others, we might choose to use 2 or 3 bytes to divide the key space, so that
	// the redaer worker can concurrently iterate keys with the same prefix bytes (same table).
	ReaderShardPrefixBytes int
}

type KVPair struct {
	Key   []byte
	Value []byte
}

func GeneratePrefixes(n int) [][]byte {
	if n == 0 {
		return [][]byte{{}}
	}
	var results [][]byte
	base := 1 << (8 * n)
	for i := 0; i < base; i++ {
		buf := make([]byte, n)
		switch n {
		case 1:
			buf[0] = byte(i)
		case 2:
			binary.BigEndian.PutUint16(buf, uint16(i))
		case 3:
			buf[0] = byte(i >> 16)
			buf[1] = byte(i >> 8)
			buf[2] = byte(i)
		default:
			panic("unsupported prefix byte length")
		}
		results = append(results, buf)
	}
	return results
}

func GenerateKeysShorterThanPrefix(n int) [][]byte {
	allKeys := make([][]byte, 0)
	for i := 1; i < n; i++ {
		keys := GeneratePrefixes(i)
		allKeys = append(allKeys, keys...)
	}
	return allKeys
}

// readerWorker reads key-value pairs from BadgerDB using a prefix iterator.
func readerWorker(
	lgProgress func(int),
	wg *sync.WaitGroup,
	db *badger.DB,
	jobs <-chan []byte,
	kvChan chan<- KVPair,
) error {
	defer wg.Done()

	for prefix := range jobs {
		defer lgProgress(1)

		err := db.View(func(txn *badger.Txn) error {
			it := txn.NewIterator(badger.DefaultIteratorOptions)
			defer it.Close()
			for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
				item := it.Item()
				key := item.KeyCopy(nil)
				val, err := item.ValueCopy(nil)
				if err != nil {
					return err
				}
				kvChan <- KVPair{Key: key, Value: val}
			}

			return nil
		})

		if err != nil {
			return fmt.Errorf("Reader error for prefix %x: %v\n", prefix, err)
		}
	}

	return nil
}

// writerWorker writes key-value pairs to PebbleDB in batches.
func writerWorker(wg *sync.WaitGroup, db *pebble.DB, kvChan <-chan KVPair, batchSize int) error {
	defer wg.Done()
	batch := db.NewBatch()
	var size int

	flush := func() error {
		if err := batch.Commit(nil); err != nil {
			return fmt.Errorf("fail to commit batch: %w", err)
		}
		batch = db.NewBatch()
		size = 0
		return nil
	}

	for kv := range kvChan {
		if err := batch.Set(kv.Key, kv.Value, nil); err != nil {
			return fmt.Errorf("fail to set key value for key %x: %w", kv.Key, err)
		}
		size += len(kv.Key) + len(kv.Value)
		if size >= batchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if size > 0 {
		if err := flush(); err != nil {
			return err
		}
	}
	return nil
}

// CopyFromBadgerToPebble migrates all key-value pairs from a BadgerDB instance to a PebbleDB instance.
//
// The migration is performed in parallel using a configurable number of reader and writer workers.
// Reader workers iterate over the BadgerDB by sharded key prefixes (based on ReaderShardPrefixBytes)
// and send key-value pairs to a shared channel. Writer workers consume from this channel and write
// batched entries into PebbleDB.
//
// Configuration is provided via MigrationConfig:
//   - BatchByteSize: maximum size in bytes for a single Pebble write batch.
//   - ReaderWorkerCount: number of concurrent workers reading from Badger.
//   - WriterWorkerCount: number of concurrent workers writing to Pebble.
//   - ReaderShardPrefixBytes: number of bytes used to shard the keyspace for parallel iteration.
//
// The function blocks until all keys are migrated and written successfully.
// It returns an error if any part of the process fails.
func CopyFromBadgerToPebble(badgerDB *badger.DB, pebbleDB *pebble.DB, cfg MigrationConfig) error {
	// Step 1: Copy all keys that is shorter than cfg.ReaderShardPrefixBytes
	// for example, if cfg.ReaderShardPrefixBytes is 2, we will copy all keys that is only 1 byte.
	// this is necessary, because the next step will use the 2 bytes prefix to iterate the keys, which
	// will not cover keys that is shorter than 2 bytes.

	keysShorterThanPrefix := GenerateKeysShorterThanPrefix(cfg.ReaderShardPrefixBytes)
	err := copyExactKeysFromBadgerToPebble(badgerDB, pebbleDB, keysShorterThanPrefix)
	if err != nil {
		return fmt.Errorf("failed to copy keys shorter than prefix: %w", err)
	}

	// Step 1: Generate key prefixes for sharding
	prefixes := GeneratePrefixes(cfg.ReaderShardPrefixBytes)

	// Step 2: Start reader workers
	// Job queue for prefix scan tasks
	prefixJobs := make(chan []byte, len(prefixes))
	for _, prefix := range prefixes {
		prefixJobs <- prefix
	}
	close(prefixJobs)

	kvChan := make(chan KVPair, 1000)

	// Reader worker pool
	lg := util.LogProgress(
		log.Logger,
		util.DefaultLogProgressConfig("migration keys from badger to pebble", len(prefixes)),
	)

	// Error collection
	errChan := make(chan error, cfg.ReaderWorkerCount+cfg.WriterWorkerCount)

	var readerWg sync.WaitGroup
	for i := 0; i < cfg.ReaderWorkerCount; i++ {
		readerWg.Add(1)
		go func() {
			// Each reader worker will take a prefix from the channel and read all keys with that prefix
			// from BadgerDB, sending the key-value pairs to the kvChan.
			err := readerWorker(lg, &readerWg, badgerDB, prefixJobs, kvChan)
			errChan <- err
		}()
	}

	// Step 3: Start writer workers
	var writerWg sync.WaitGroup
	for i := 0; i < cfg.WriterWorkerCount; i++ {
		writerWg.Add(1)
		go func() {
			// Each writer worker will take key-value pairs from the kvChan and write them to PebbleDB
			// in batches of size cfg.BatchByteSize (32MB by default).
			err := writerWorker(&writerWg, pebbleDB, kvChan, cfg.BatchByteSize)
			errChan <- err
		}()
	}

	// Wait for all readers to finish, then close writer channel
	go func() {
		readerWg.Wait()
		close(kvChan)
	}()

	writerWg.Wait()

	// Check for errors
	close(errChan)
	for err := range errChan {
		if err != nil {
			return fmt.Errorf("error: %w", err)
		}
	}

	return nil
}

func copyExactKeysFromBadgerToPebble(badgerDB *badger.DB, pebbleDB *pebble.DB, keys [][]byte) error {
	batch := pebbleDB.NewBatch()
	for _, key := range keys {
		err := badgerDB.View(func(txn *badger.Txn) error {
			item, err := txn.Get(key)
			if err != nil {
				if errors.Is(err, badger.ErrKeyNotFound) {
					// skip if the key is not found
					return nil
				}

				return err
			}

			// read key value
			val, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}

			// write to pebble
			err = batch.Set(key, val, nil)
			if err != nil {
				return fmt.Errorf("failed to set key %x in PebbleDB: %w", key, err)
			}

			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to get key %x from BadgerDB: %w", key, err)
		}
	}

	err := batch.Commit(nil)
	if err != nil {
		return fmt.Errorf("failed to commit batch to PebbleDB: %w", err)
	}

	return nil
}
