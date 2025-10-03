package migration

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/objstorage/objstorageprovider"
	"github.com/cockroachdb/pebble/v2/sstable"
	"github.com/cockroachdb/pebble/v2/vfs"
	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/module/util"
)

// CopyFromBadgerToPebble copies all key-value pairs from a BadgerDB to a PebbleDB
// using SSTable ingestion. It reads BadgerDB in prefix-sharded ranges and writes
// those ranges into SSTable files, which are then ingested into Pebble.
func CopyFromBadgerToPebbleSSTables(badgerDB *badger.DB, pebbleDB *pebble.DB, cfg MigrationConfig) error {
	sstableDir, err := os.MkdirTemp(cfg.PebbleDir, "flow-migration-temp-")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}

	log.Info().Msgf("Created temporary directory for SSTables: %s", sstableDir)

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
	keyCount, err := copyExactKeysFromBadgerToPebble(badgerDB, pebbleDB, keysShorterThanPrefix)
	if err != nil {
		return fmt.Errorf("failed to copy keys shorter than prefix: %w", err)
	}
	log.Info().Msgf("Copied %d keys shorter than %v bytes prefix", keyCount, cfg.ReaderShardPrefixBytes)

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
			if err := writerSSTableWorker(ctx, i, pebbleDB, sstableDir, kvChan); err != nil {
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

func writerSSTableWorker(ctx context.Context, workerIndex int, db *pebble.DB, sstableDir string, kvChan <-chan KVPairs) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case kvGroup, ok := <-kvChan:
			if !ok {
				return nil
			}

			filePath := fmt.Sprintf("%s/prefix_%x_worker_%v.sst", sstableDir, kvGroup.Prefix, workerIndex)
			writer, err := createSSTableWriter(filePath)
			if err != nil {
				return err
			}

			for _, kv := range kvGroup.Pairs {
				if err := writer.Set(kv.Key, kv.Value); err != nil {
					return fmt.Errorf("fail to set key %x: %w", kv.Key, err)
				}
			}

			if err := writer.Close(); err != nil {
				return fmt.Errorf("fail to close writer: %w", err)
			}

			err = db.Ingest(ctx, []string{filePath})
			if err != nil {
				return fmt.Errorf("fail to ingest file %v: %w", filePath, err)
			}

			log.Info().Msgf("Ingested SSTable file: %s", filePath)
		}
	}
}
func createSSTableWriter(filePath string) (*sstable.Writer, error) {
	f, err := vfs.Default.Create(filePath, vfs.WriteCategoryUnspecified)
	if err != nil {
		return nil, err
	}

	writable := objstorageprovider.NewFileWritable(f)
	sstWriter := sstable.NewWriter(writable, sstable.WriterOptions{
		// pebble 1 is using TableFormatPebblev4, pebble 2's latest is TableFormatPebblev5 (TableFormatMax)
		// in order to be compatible with pebble 1, we use TableFormatPebblev4 for now.
		// TODO: use TableFormatMax in next spork
		// TableFormat: sstable.TableFormatMax,
		TableFormat: sstable.TableFormatPebblev4,
	})

	return sstWriter, nil
}

func ForceCompactPebbleDB(pebbleDir string) error {
	pebbleDB, err := pebble.Open(pebbleDir, &pebble.Options{
		// TODO: use FormatNewest in next spork
		// FormatMajorVersion: pebble.FormatNewest,
		FormatMajorVersion: pebble.FormatVirtualSSTables,
	})
	if err != nil {
		return err
	}

	return pebbleDB.Compact(context.TODO(), []byte{0x00}, []byte{0xff}, true)
}
