package migration

import (
	"context"
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
	PebbleDir         string
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

type KVPairs struct {
	Prefix []byte
	Pairs  []KVPair
}

type KVPair struct {
	Key   []byte
	Value []byte
}

func GeneratePrefixes(n int) [][]byte {
	if n == 0 {
		return [][]byte{{}}
	}

	base := 1 << (8 * n)
	results := make([][]byte, 0, base)

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
	ctx context.Context,
	lgProgress func(int),
	db *badger.DB,
	jobs <-chan []byte, // each job is a prefix to iterate over
	kvChan chan<- KVPairs, // channel to send key-value pairs to writer workers
	batchSize int,
) error {
	for prefix := range jobs {
		err := db.View(func(txn *badger.Txn) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			options := badger.DefaultIteratorOptions
			options.Prefix = prefix
			it := txn.NewIterator(options)
			defer it.Close()

			var (
				kvBatch  []KVPair
				currSize int
			)

			for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
				if ctx.Err() != nil {
					return ctx.Err()
				}

				item := it.Item()
				key := item.KeyCopy(nil)
				val, err := item.ValueCopy(nil)
				if err != nil {
					return err
				}

				kvBatch = append(kvBatch, KVPair{Key: key, Value: val})
				currSize += len(key) + len(val)

				if currSize >= batchSize {
					select {
					case kvChan <- KVPairs{Prefix: prefix, Pairs: kvBatch}:
					case <-ctx.Done():
						return ctx.Err()
					}
					kvBatch = nil
					currSize = 0
				}
			}

			if len(kvBatch) > 0 {
				select {
				case kvChan <- KVPairs{Prefix: prefix, Pairs: kvBatch}:
				case <-ctx.Done():
					return ctx.Err()
				}
			}

			return nil
		})

		lgProgress(1)

		if err != nil {
			return err
		}
	}
	return nil
}

// writerWorker writes key-value pairs to PebbleDB in batches.
func writerWorker(ctx context.Context, db *pebble.DB, kvChan <-chan KVPairs) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case kvGroup, ok := <-kvChan:
			if !ok {
				return nil
			}
			batch := db.NewBatch()
			for _, kv := range kvGroup.Pairs {
				if err := batch.Set(kv.Key, kv.Value, nil); err != nil {
					return fmt.Errorf("fail to set key %x: %w", kv.Key, err)
				}
			}

			if err := batch.Commit(nil); err != nil {
				return fmt.Errorf("fail to commit batch: %w", err)
			}
		}
	}
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		errOnce  sync.Once
		firstErr error
	)

	// once running into an exception, cancel the context and report the first error
	reportFirstError := func(err error) {
		if err != nil {
			errOnce.Do(func() {
				firstErr = err
				cancel()
			})
		}
	}

	// Step 1: Copy all keys shorter than prefix
	keysShorterThanPrefix := GenerateKeysShorterThanPrefix(cfg.ReaderShardPrefixBytes)
	if err := copyExactKeysFromBadgerToPebble(badgerDB, pebbleDB, keysShorterThanPrefix); err != nil {
		return fmt.Errorf("failed to copy keys shorter than prefix: %w", err)
	}
	log.Info().Msgf("Copied %d keys shorter than %v bytes prefix", len(keysShorterThanPrefix), cfg.ReaderShardPrefixBytes)

	// Step 2: Copy all keys with prefix by first generating prefix shards and then
	// using reader and writer workers to copy the keys with the same prefix
	prefixes := GeneratePrefixes(cfg.ReaderShardPrefixBytes)
	prefixJobs := make(chan []byte, len(prefixes))
	for _, prefix := range prefixes {
		prefixJobs <- prefix
	}
	close(prefixJobs)

	kvChan := make(chan KVPairs, cfg.ReaderWorkerCount*2)

	lg := util.LogProgress(
		log.Logger,
		util.DefaultLogProgressConfig("migration keys from badger to pebble", len(prefixes)),
	)

	var readerWg sync.WaitGroup
	for i := 0; i < cfg.ReaderWorkerCount; i++ {
		readerWg.Add(1)
		go func() {
			defer readerWg.Done()
			if err := readerWorker(ctx, lg, badgerDB, prefixJobs, kvChan, cfg.BatchByteSize); err != nil {
				reportFirstError(err)
			}
		}()
	}

	var writerWg sync.WaitGroup
	for i := 0; i < cfg.WriterWorkerCount; i++ {
		writerWg.Add(1)
		go func() {
			defer writerWg.Done()
			if err := writerWorker(ctx, pebbleDB, kvChan); err != nil {
				reportFirstError(err)
			}
		}()
	}

	// Close kvChan after readers complete
	go func() {
		readerWg.Wait()
		close(kvChan)
	}()

	writerWg.Wait()
	return firstErr
}

func copyExactKeysFromBadgerToPebble(badgerDB *badger.DB, pebbleDB *pebble.DB, keys [][]byte) error {
	batch := pebbleDB.NewBatch()
	err := badgerDB.View(func(txn *badger.Txn) error {
		for _, key := range keys {
			item, err := txn.Get(key)
			if err != nil {
				if errors.Is(err, badger.ErrKeyNotFound) {
					// skip if the key is not found
					continue
				}

				return err
			}

			err = item.Value(func(val []byte) error {
				return batch.Set(key, val, nil)
			})

			if err != nil {
				return fmt.Errorf("failed to get value for key %x: %w", key, err)
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to get key from BadgerDB: %w", err)
	}

	err = batch.Commit(pebble.Sync)
	if err != nil {
		return fmt.Errorf("failed to commit batch to PebbleDB: %w", err)
	}

	return nil
}
