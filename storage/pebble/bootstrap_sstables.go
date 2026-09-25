package pebble

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/objstorage/objstorageprovider"
	"github.com/cockroachdb/pebble/v2/sstable"
	"github.com/cockroachdb/pebble/v2/vfs"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/storage/pebble/registers"
)

const (
	// registerBootstrapBucketCount is the number of buckets the registers are sharded into
	// while bootstrapping from a checkpoint. Registers are assigned to a bucket by the first
	// byte of their owner, i.e. the first byte of the lookup key after the code prefix. All
	// keys of a bucket therefore sort before all keys of any higher bucket, which makes the
	// key ranges of the ingested sstables non-overlapping.
	registerBootstrapBucketCount = 1 << 8

	// registerBootstrapTempDirPrefix is the prefix of the temporary directory, created
	// inside the register store directory, that holds the bucket files and sstables of an
	// ongoing bootstrap. It is removed when the bootstrap completes (successfully or not).
	registerBootstrapTempDirPrefix = "register-bootstrap-sstables-"

	// registerBootstrapBucketBufSize is the write buffer size of each bucket file. All
	// buckets are written concurrently, so the buffers of all buckets are held in memory.
	registerBootstrapBucketBufSize = 256 << 10

	// registerBootstrapReadBufSize is the read buffer size used to read a bucket file.
	registerBootstrapReadBufSize = 1 << 20

	// registerBootstrapTableFormat is the sstable format of the ingested sstables. Pebble 2
	// writes TableFormatMax, but v4 is the newest format understood by pebble 1, which
	// allows rolling back to a pebble 1 binary (same choice as the storage migration
	// sstables, see storage/migration/sstables.go).
	registerBootstrapTableFormat = sstable.TableFormatPebblev4

	// registerBootstrapMaxRecordSize bounds the size of a single key or value read from a
	// bucket file. Bucket files are written by this process and only read back by this
	// process, so this only protects against a corrupted file causing a huge allocation.
	registerBootstrapMaxRecordSize = 64 << 20
)

// registerBootstrapRecord is a register read back from a bucket file.
type registerBootstrapRecord struct {
	key   []byte
	value []byte
}

// registerBootstrapBucketFile describes a bucket file written during the shard phase.
type registerBootstrapBucketFile struct {
	// bucket is the index of the bucket, which is also the sort order of its registers.
	bucket int
	path   string
	// size is the size of the bucket file in bytes, which is roughly the size of the
	// registers it holds.
	size int64
}

// RegisterBootstrapSSTables bootstraps an empty register store from a compacted single-trie
// V6 checkpoint by writing the checkpoint's registers to sstables and ingesting them into
// pebble, instead of writing them with batched writes.
//
// Batching writes goes through the pebble write path (write-ahead log, memtable, flush and
// eventually compaction into the lowest level), which rewrites the whole register store
// several times for a store that, at bootstrap, is written once and never overwritten. An
// empty pebble store ingests non-overlapping sstables directly into its lowest level, so
// ingesting the register store as a set of sstables writes it exactly once.
//
// Pebble requires ingested sstables to be sorted by the database's comparer and to not
// overlap, while the checkpoint emits its leaf nodes in trie path order, i.e. in the order
// of the hashes of the register keys, which is unrelated to the lookup key order. The
// registers are therefore sharded into bucket files by owner prefix:
//
//  1. all leaf nodes of the checkpoint are read once and appended to the bucket file of
//     their owner prefix;
//  2. the bucket files are read back in key order, grouped into batches of at least the
//     lowest level's target file size, sorted by lookup key and written to sstables, which
//     are ingested into pebble with a single ingestion.
//
// Because each bucket is sorted on its own, the peak memory of a bootstrap is the size of
// the largest bucket (about 1/registerBootstrapBucketCount of the register data) plus the
// overhead of sorting it, times the number of workers. The temporary bucket files and
// sstables require as much free space as the register data in the register store directory,
// on the same file system, so that pebble can link the sstables instead of copying them.
// The bootstrap keeps one file open per bucket (registerBootstrapBucketCount file
// descriptors), so the file descriptor limit has to accommodate it.
//
// NOT CONCURRENCY SAFE! IndexCheckpointFile can only be called once.
type RegisterBootstrapSSTables struct {
	log                zerolog.Logger
	db                 *pebble.DB
	registerDir        string
	checkpointDir      string
	checkpointFileName string
	leafNodeChan       chan *wal.LeafNode
	rootHeight         uint64
	rootHash           ledger.RootHash

	// sstableWriterOptions are derived from the options the register store is opened with,
	// so that the ingested sstables use the same comparer (registers.NewMVCCComparer), the
	// same bloom filters and the same block sizes as the sstables pebble writes itself.
	sstableWriterOptions sstable.WriterOptions
	// sstableTargetFileSize is the target file size of the lowest level, which is the size
	// the ingested sstables are split at.
	sstableTargetFileSize uint64

	registerCount uint64
	sstableCount  int
}

// NewRegisterBootstrapSSTables creates a bootstrapper that writes the registers of the given
// checkpoint to sstables and ingests them into the given register store.
//
// The given register store directory must be the directory the database is stored in, and it
// must have enough free space left for the temporary bucket files and sstables. The caller
// must ensure that the database has not been bootstrapped yet; this is checked here.
//
// No error returns are expected during normal operation.
func NewRegisterBootstrapSSTables(
	db *pebble.DB,
	registerDir string,
	checkpointFile string,
	rootHeight uint64,
	rootHash ledger.RootHash,
	log zerolog.Logger,
) (*RegisterBootstrapSSTables, error) {
	isBootstrapped, err := IsBootstrapped(db)
	if err != nil {
		return nil, err
	}
	if isBootstrapped {
		// key detected, attempt to run bootstrap on corrupt or already bootstrapped data
		return nil, ErrAlreadyBootstrapped
	}

	// The writer options are derived from the options the register store is opened with.
	// The comparer in particular must match: pebble refuses to read an ingested sstable
	// whose comparer name is unknown to the database.
	cache := pebble.NewCache(DefaultPebbleCacheSize)
	defer cache.Unref()
	opts := DefaultPebbleOptions(log, cache, registers.NewMVCCComparer())
	// An ingested sstable is placed in the lowest level the LSM has no overlap with. For an
	// empty register store that is the bottom level, so the sstables are configured as if
	// pebble itself wrote them to that level.
	lowestLevel := len(opts.Levels) - 1

	checkpointDir, checkpointFileName := filepath.Split(checkpointFile)
	return &RegisterBootstrapSSTables{
		log:                  log.With().Str("module", "register_bootstrap_sstables").Logger(),
		db:                   db,
		registerDir:          registerDir,
		checkpointDir:        checkpointDir,
		checkpointFileName:   checkpointFileName,
		leafNodeChan:         make(chan *wal.LeafNode, checkpointLeafNodeBufSize),
		rootHeight:           rootHeight,
		rootHash:             rootHash,
		sstableWriterOptions: opts.MakeWriterOptions(lowestLevel, registerBootstrapTableFormat),
		sstableTargetFileSize: uint64(
			opts.Levels[lowestLevel].TargetFileSize,
		),
	}, nil
}

// IndexCheckpointFile indexes the registers of the checkpoint file into the register store
// by ingesting them as sstables, and initializes the register store's first and latest
// height to the bootstrap height.
//
// The database is not usable as a register store until this method returns successfully. If
// it fails, the register store directory has to be deleted before retrying, as some of the
// registers may already have been ingested.
//
// `workerCount` (at least 1) groups of bucket files are sorted and written to sstables in
// parallel. The peak memory of a bootstrap is roughly `workerCount` times the larger of the
// target sstable size and the largest bucket (about 1/registerBootstrapBucketCount of the
// register data).
//
// Expected error returns during normal operation:
//   - [context.Canceled], [context.DeadlineExceeded]: if the context is cancelled
func (b *RegisterBootstrapSSTables) IndexCheckpointFile(ctx context.Context, workerCount int) error {
	if workerCount < 1 {
		return fmt.Errorf("worker count must be at least 1, got %d", workerCount)
	}

	start := time.Now()

	tempDir, err := os.MkdirTemp(b.registerDir, registerBootstrapTempDirPrefix)
	if err != nil {
		return fmt.Errorf("could not create temporary directory in %s: %w", b.registerDir, err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			b.log.Error().Err(removeErr).Msgf("could not remove temporary directory %s", tempDir)
		}
	}()

	b.log.Info().Msgf("sharding checkpoint registers into %v bucket files", registerBootstrapBucketCount)
	shardStart := time.Now()
	buckets, err := b.writeBucketFiles(ctx, tempDir)
	if err != nil {
		return fmt.Errorf("could not write checkpoint registers to bucket files: %w", err)
	}
	shardDuration := time.Since(shardStart)

	b.log.Info().Msgf("writing sstables for %v buckets with %v workers", len(buckets), workerCount)
	sstableStart := time.Now()
	sstables, err := b.writeSSTables(ctx, tempDir, buckets, workerCount)
	if err != nil {
		return fmt.Errorf("could not write sstables: %w", err)
	}
	sstableDuration := time.Since(sstableStart)

	// Ingesting into an empty register store places all sstables in its lowest level,
	// without any flush or compaction: the registers are written exactly once.
	ingestStart := time.Now()
	if len(sstables) > 0 {
		if err := b.db.Ingest(ctx, sstables); err != nil {
			return fmt.Errorf("could not ingest %d sstables: %w", len(sstables), err)
		}
	}
	ingestDuration := time.Since(ingestStart)

	if err := initHeights(b.db, b.rootHeight); err != nil {
		return fmt.Errorf("could not index latest height: %w", err)
	}

	b.log.Info().
		Uint64("root_height", b.rootHeight).
		Uint64("register_count", b.registerCount).
		Int("bucket_count", len(buckets)).
		Int("sstable_count", b.sstableCount).
		// note: not using Dur() since default units are ms and these durations are long
		Str("shard_duration", fmt.Sprintf("%v", shardDuration)).
		Str("sstable_duration", fmt.Sprintf("%v", sstableDuration)).
		Str("ingest_duration", fmt.Sprintf("%v", ingestDuration)).
		Str("duration", fmt.Sprintf("%v", time.Since(start))).
		Msg("checkpoint indexing complete")

	return nil
}

// writeBucketFiles reads the leaf nodes of the checkpoint and writes each register to the
// bucket file of its owner prefix. It returns the bucket files that contain at least one
// register, in ascending lookup key order.
//
// NOT CONCURRENCY SAFE!
//
// No error returns are expected during normal operation.
func (b *RegisterBootstrapSSTables) writeBucketFiles(ctx context.Context, tempDir string) ([]registerBootstrapBucketFile, error) {
	bucketFiles := make([]*bucketFile, registerBootstrapBucketCount)
	defer func() {
		for _, bucketFile := range bucketFiles {
			if bucketFile == nil {
				continue
			}
			if closeErr := bucketFile.Close(); closeErr != nil {
				b.log.Error().Err(closeErr).Msgf("could not close bucket file %s", bucketFile.path)
			}
		}
	}()

	// The reader pushes leaf nodes to the channel and closes it when it is done, so it has
	// to run in its own goroutine while this goroutine consumes the channel.
	readErrCh := make(chan error, 1)
	go func() {
		readErrCh <- wal.OpenAndReadLeafNodesFromCheckpointV6(
			b.leafNodeChan, b.checkpointDir, b.checkpointFileName, b.rootHash, b.log)
	}()

	var shardErr error
	for leafNode := range b.leafNodeChan {
		if shardErr != nil {
			// Keep draining the channel, otherwise the reader blocks and never returns.
			// The bootstrap is aborted anyway: the caller has to delete the register
			// store directory before retrying.
			continue
		}
		if err := ctx.Err(); err != nil {
			shardErr = err
			continue
		}
		shardErr = b.writeLeafNodeToBucket(bucketFiles, tempDir, leafNode)
	}

	if err := <-readErrCh; err != nil {
		return nil, fmt.Errorf("could not read checkpoint file %s: %w", b.checkpointFileName, err)
	}
	if shardErr != nil {
		return nil, shardErr
	}

	buckets := make([]registerBootstrapBucketFile, 0, registerBootstrapBucketCount)
	for i, bucketFile := range bucketFiles {
		if bucketFile == nil {
			continue
		}
		if err := bucketFile.Close(); err != nil {
			return nil, fmt.Errorf("could not close bucket file %s: %w", bucketFile.path, err)
		}
		size, err := bucketFile.size()
		if err != nil {
			return nil, err
		}
		buckets = append(buckets, registerBootstrapBucketFile{bucket: i, path: bucketFile.path, size: size})
	}
	return buckets, nil
}

// writeLeafNodeToBucket appends the register of the given leaf node to the bucket file of
// its owner prefix, creating that bucket file if it is the first register of the bucket.
//
// NOT CONCURRENCY SAFE!
//
// No error returns are expected during normal operation.
func (b *RegisterBootstrapSSTables) writeLeafNodeToBucket(
	bucketFiles []*bucketFile,
	tempDir string,
	leafNode *wal.LeafNode,
) error {
	key, err := leafNode.Payload.Key()
	if err != nil {
		return fmt.Errorf("could not get key from register payload: %w", err)
	}

	registerID, err := convert.LedgerKeyToRegisterID(key)
	if err != nil {
		return fmt.Errorf("could not get register ID from key: %w", err)
	}

	lookupKey := newLookupKey(b.rootHeight, registerID).Bytes()
	bucket := registerBootstrapBucket(lookupKey)

	if bucketFiles[bucket] == nil {
		bucketFile, err := createBucketFile(tempDir, bucket)
		if err != nil {
			return err
		}
		bucketFiles[bucket] = bucketFile
	}

	if err := bucketFiles[bucket].write(lookupKey, leafNode.Payload.Value()); err != nil {
		return fmt.Errorf("could not write register to bucket file %s: %w", bucketFiles[bucket].path, err)
	}
	return nil
}

// writeSSTables reads the registers of the given bucket files, sorts each group of
// consecutive buckets by lookup key and writes the sorted registers to sstables, with up to
// `workerCount` groups in parallel. It returns the paths of all sstables it wrote.
//
// Consecutive buckets are grouped until the group is at least as large as the target sstable
// size, which keeps a group in memory bounded by the larger of the target sstable size and
// the largest bucket, and avoids writing many tiny sstables when the register data is small.
// The groups partition the buckets, so the key ranges of all sstables are non-overlapping
// and sorted, and a group does not have to be sorted in any particular order relative to the
// other groups.
//
// NOT CONCURRENCY SAFE!
//
// No error returns are expected during normal operation.
func (b *RegisterBootstrapSSTables) writeSSTables(
	ctx context.Context,
	dir string,
	buckets []registerBootstrapBucketFile,
	workerCount int,
) ([]string, error) {
	var (
		mutex    sync.Mutex
		sstables []string
	)

	g, _ := errgroup.WithContext(ctx)
	g.SetLimit(workerCount)
	for groupStart := 0; groupStart < len(buckets); {
		groupEnd := bucketGroupEnd(buckets, groupStart, int64(b.sstableTargetFileSize))
		group := buckets[groupStart:groupEnd]
		groupStart = groupEnd

		g.Go(func() error {
			records, err := readBucketFiles(group)
			if err != nil {
				return err
			}

			slices.SortFunc(records, func(a, b registerBootstrapRecord) int {
				return bytes.Compare(a.key, b.key)
			})

			groupSSTables, err := b.writeSortedRecords(dir, group[0].bucket, records)
			if err != nil {
				return err
			}

			b.log.Debug().
				Int("bucket", group[0].bucket).
				Int("register_count", len(records)).
				Int("sstable_count", len(groupSSTables)).
				Msg("wrote sstables for bucket group")

			mutex.Lock()
			defer mutex.Unlock()
			b.registerCount += uint64(len(records))
			b.sstableCount += len(groupSSTables)
			sstables = append(sstables, groupSSTables...)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return sstables, nil
}

// bucketGroupEnd returns the index of the first bucket after the group that starts at
// `start`: consecutive buckets are added to the group while its total size is below
// `targetBytes`, so that a group holds at least `targetBytes` registers (unless the
// remaining buckets are smaller) without holding more than one oversized bucket extra.
func bucketGroupEnd(buckets []registerBootstrapBucketFile, start int, targetBytes int64) int {
	end := start + 1
	var groupBytes int64 = buckets[start].size
	for end < len(buckets) && groupBytes < targetBytes {
		groupBytes += buckets[end].size
		end++
	}
	return end
}

// writeSortedRecords writes the given registers, sorted by lookup key, to sstables of at most
// the lowest level's target file size, and returns the paths of the sstables it wrote. The
// sstables are named after the given bucket.
//
// NOT CONCURRENCY SAFE!
//
// No error returns are expected during normal operation.
func (b *RegisterBootstrapSSTables) writeSortedRecords(
	dir string,
	bucket int,
	records []registerBootstrapRecord,
) ([]string, error) {
	var (
		sstables []string
		writer   *sstable.Writer
	)

	// Closing the sstable writer flushes, syncs and closes the underlying file, which is
	// required before the sstable can be ingested.
	closeWriter := func() error {
		if writer == nil {
			return nil
		}
		err := writer.Close()
		writer = nil
		return err
	}
	defer func() {
		if closeErr := closeWriter(); closeErr != nil {
			b.log.Error().Err(closeErr).Msg("could not close sstable writer")
		}
	}()

	for _, record := range records {
		if writer != nil && writer.Raw().EstimatedSize() >= b.sstableTargetFileSize {
			if err := closeWriter(); err != nil {
				return nil, fmt.Errorf("could not close sstable: %w", err)
			}
		}

		if writer == nil {
			path := filepath.Join(dir, fmt.Sprintf("%s-%04d.sst", bucketFileName(bucket), len(sstables)))
			file, err := vfs.Default.Create(path, vfs.WriteCategoryUnspecified)
			if err != nil {
				return nil, err
			}
			writer = sstable.NewWriter(objstorageprovider.NewFileWritable(file), b.sstableWriterOptions)
			sstables = append(sstables, path)
		}

		if err := writer.Set(record.key, record.value); err != nil {
			return nil, fmt.Errorf("could not write register to sstable: %w", err)
		}
	}

	if err := closeWriter(); err != nil {
		return nil, fmt.Errorf("could not close sstable: %w", err)
	}
	return sstables, nil
}

// registerBootstrapBucket returns the bucket a register belongs to, which is the first byte
// of its owner: the first byte of the lookup key after the register code byte. Global
// registers (empty owner) are assigned to the bucket of the '/' separator that follows the
// owner.
func registerBootstrapBucket(lookupKey []byte) int {
	// lookup keys are at least MinLookupKeyLen bytes long
	return int(lookupKey[1])
}

// bucketFileName returns the file name of the given bucket.
func bucketFileName(bucket int) string {
	return fmt.Sprintf("bucket-%03d", bucket)
}

// bucketFile is the write end of a bucket file: a buffered append-only file of registers,
// each encoded as a length-prefixed key followed by a length-prefixed value.
type bucketFile struct {
	path   string
	file   *os.File
	writer *bufio.Writer
}

// createBucketFile creates the bucket file of the given bucket in the given directory.
//
// No error returns are expected during normal operation.
func createBucketFile(dir string, bucket int) (*bucketFile, error) {
	path := filepath.Join(dir, bucketFileName(bucket))
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("could not create bucket file %s: %w", path, err)
	}

	return &bucketFile{
		path:   path,
		file:   file,
		writer: bufio.NewWriterSize(file, registerBootstrapBucketBufSize),
	}, nil
}

// write appends the given key and value to the bucket file.
//
// No error returns are expected during normal operation.
func (b *bucketFile) write(key []byte, value []byte) error {
	var lenBuf [binary.MaxVarintLen64]byte

	n := binary.PutUvarint(lenBuf[:], uint64(len(key)))
	if _, err := b.writer.Write(lenBuf[:n]); err != nil {
		return err
	}
	if _, err := b.writer.Write(key); err != nil {
		return err
	}

	n = binary.PutUvarint(lenBuf[:], uint64(len(value)))
	if _, err := b.writer.Write(lenBuf[:n]); err != nil {
		return err
	}
	_, err := b.writer.Write(value)
	return err
}

// size returns the size of the closed bucket file in bytes.
//
// No error returns are expected during normal operation.
func (b *bucketFile) size() (int64, error) {
	info, err := os.Stat(b.path)
	if err != nil {
		return 0, fmt.Errorf("could not stat bucket file %s: %w", b.path, err)
	}
	return info.Size(), nil
}

// Close flushes and closes the bucket file. It is a no-op if the file is already closed.
//
// No error returns are expected during normal operation.
func (b *bucketFile) Close() error {
	if b.file == nil {
		return nil
	}
	flushErr := b.writer.Flush()
	closeErr := b.file.Close()
	b.file = nil
	return errors.Join(flushErr, closeErr)
}

// readBucketFiles reads all registers of the given bucket files, in the given order.
//
// No error returns are expected during normal operation.
func readBucketFiles(buckets []registerBootstrapBucketFile) ([]registerBootstrapRecord, error) {
	var records []registerBootstrapRecord
	for _, bucket := range buckets {
		bucketRecords, err := readBucketFile(bucket.path)
		if err != nil {
			return nil, err
		}
		records = append(records, bucketRecords...)
	}
	return records, nil
}

// readBucketFile reads all registers of the given bucket file.
//
// No error returns are expected during normal operation.
func readBucketFile(path string) ([]registerBootstrapRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open bucket file %s: %w", path, err)
	}
	defer func() {
		_ = file.Close()
	}()

	reader := bufio.NewReaderSize(file, registerBootstrapReadBufSize)
	var records []registerBootstrapRecord
	for {
		key, err := readBucketRecordPart(reader, path)
		if errors.Is(err, io.EOF) {
			return records, nil
		}
		if err != nil {
			return nil, err
		}

		value, err := readBucketRecordPart(reader, path)
		if err != nil {
			return nil, fmt.Errorf("truncated bucket file %s: %w", path, err)
		}

		records = append(records, registerBootstrapRecord{key: key, value: value})
	}
}

// readBucketRecordPart reads a length-prefixed key or value from the given bucket file
// reader. It returns [io.EOF] when the reader is at a record boundary.
//
// No error returns are expected during normal operation.
func readBucketRecordPart(reader *bufio.Reader, path string) ([]byte, error) {
	size, err := binary.ReadUvarint(reader)
	if err != nil {
		return nil, err
	}
	if size > registerBootstrapMaxRecordSize {
		return nil, fmt.Errorf("corrupted bucket file %s: record of %d bytes exceeds the limit of %d bytes",
			path, size, registerBootstrapMaxRecordSize)
	}

	part := make([]byte, size)
	if _, err := io.ReadFull(reader, part); err != nil {
		return nil, err
	}
	return part, nil
}
