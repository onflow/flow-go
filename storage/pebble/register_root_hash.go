package pebble

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/bitutils"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/model/flow"
)

const (
	// registerRootHashBucketBits is the number of leading bits of a register's path that select
	// the bucket file the register's path and leaf hash are written to. Each bucket holds the
	// registers of one sub tree of the merkle trie, so the buckets can be hashed independently
	// and bound the memory the computation needs.
	registerRootHashBucketBits = 8

	// registerRootHashBucketCount is the number of bucket files.
	registerRootHashBucketCount = 1 << registerRootHashBucketBits

	// registerRootHashBucketHeight is the height of the trie node covering all paths of a
	// bucket: paths are [ledger.NodeMaxHeight] bits long and a bucket selects their first
	// registerRootHashBucketBits bits.
	registerRootHashBucketHeight = ledger.NodeMaxHeight - registerRootHashBucketBits

	// registerRootHashRecordLen is the length of a bucket file record: the register's path
	// followed by the hash of its leaf.
	registerRootHashRecordLen = 2 * hash.HashLen

	// registerRootHashBucketBufSize is the write buffer size of each bucket file.
	registerRootHashBucketBufSize = 256 << 10

	// registerRootHashTempDirPrefix is the prefix of the temporary directory holding the bucket
	// files of an ongoing computation.
	registerRootHashTempDirPrefix = "register-root-hash-"
)

// ComputeRegisterRootHash computes the root hash of the registers of the given register store
// at the given height, which is the state commitment of the state the register store holds.
//
// The root hash is computed the same way the ledger computes it: every register is reduced to
// the path of its register ID (using the given path finder version) and the hash of its leaf,
// and those are folded into the merkle trie's root hash. Register values are not kept in
// memory: each register becomes one record of its path and leaf hash ([registerRootHashRecordLen]
// bytes) in the bucket file selected by the leading bits of its path, and the bucket files are
// then folded into subtree hashes and combined into the root hash.
//
// The registers are read with [Registers.ByKeyPrefix], so the computation validates the state
// at the given height of a register store that has heights above it, and registers that were
// removed at or before the given height are not part of the state.
//
// The given directory must have enough free space for the bucket files (one per
// [registerRootHashBucketCount] buckets, about half the size of the register data); a
// temporary directory is created in it and removed when the computation completes.
//
// Expected error returns during normal operation:
//   - [context.Canceled], [context.DeadlineExceeded]: if the context is cancelled
func ComputeRegisterRootHash(
	ctx context.Context,
	log zerolog.Logger,
	registers *Registers,
	height uint64,
	tempDir string,
	pathFinderVersion uint8,
	workerCount int,
) (ledger.RootHash, error) {
	if workerCount < 1 {
		return ledger.RootHash{}, fmt.Errorf("worker count must be at least 1, got %d", workerCount)
	}

	bucketDir, err := os.MkdirTemp(tempDir, registerRootHashTempDirPrefix)
	if err != nil {
		return ledger.RootHash{}, fmt.Errorf("could not create temporary directory in %s: %w", tempDir, err)
	}
	defer func() {
		if removeErr := os.RemoveAll(bucketDir); removeErr != nil {
			log.Error().Err(removeErr).Msgf("could not remove temporary directory %s", bucketDir)
		}
	}()

	start := time.Now()
	log.Info().Msg("writing register paths and leaf hashes to bucket files")
	pathsStart := time.Now()
	registerCount, err := writeRegisterPathBuckets(ctx, registers, height, pathFinderVersion, bucketDir)
	if err != nil {
		return ledger.RootHash{}, fmt.Errorf("could not write register paths to bucket files: %w", err)
	}
	pathsDuration := time.Since(pathsStart)

	log.Info().Msgf("folding %v registers into subtree hashes with %v workers", registerCount, workerCount)
	hashesStart := time.Now()
	subtreeHashes, err := foldRegisterPathBuckets(ctx, bucketDir, workerCount)
	if err != nil {
		return ledger.RootHash{}, fmt.Errorf("could not fold register paths into subtree hashes: %w", err)
	}
	hashesDuration := time.Since(hashesStart)

	rootHash := combineSubtreeRootHashes(subtreeHashes)

	log.Info().
		Uint64("height", height).
		Uint64("register_count", registerCount).
		// note: not using Dur() since default units are ms and these durations are long
		Str("paths_duration", fmt.Sprintf("%v", pathsDuration)).
		Str("hashes_duration", fmt.Sprintf("%v", hashesDuration)).
		Str("duration", fmt.Sprintf("%v", time.Since(start))).
		Msg("register root hash computed")

	return rootHash, nil
}

// writeRegisterPathBuckets reads the registers of the given register store at the given height
// and writes the path and the leaf hash of each register to the bucket file selected by the
// leading bits of its path. It returns the number of registers it wrote.
//
// NOT CONCURRENCY SAFE!
//
// Expected error returns during normal operation:
//   - [context.Canceled], [context.DeadlineExceeded]: if the context is cancelled
func writeRegisterPathBuckets(
	ctx context.Context,
	registers *Registers,
	height uint64,
	pathFinderVersion uint8,
	bucketDir string,
) (uint64, error) {
	bucketFiles := make([]*rootHashBucketFile, registerRootHashBucketCount)
	defer func() {
		for _, bucketFile := range bucketFiles {
			if bucketFile == nil {
				continue
			}
			if closeErr := bucketFile.Close(); closeErr != nil {
				_ = closeErr // reported by the caller's temporary directory cleanup
			}
		}
	}()

	var registerCount uint64
	for entry, err := range registers.ByKeyPrefix("", height, nil) {
		if err != nil {
			return 0, fmt.Errorf("could not scan registers: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return 0, err
		}

		value, err := entry.Value()
		if err != nil {
			return 0, fmt.Errorf("could not read register value: %w", err)
		}
		if len(value) == 0 {
			// the register was removed at or before the given height, so it is not part of the
			// state at that height
			continue
		}

		registerID := entry.Cursor()
		path, leafHash, err := registerPathAndLeafHash(registerID, value, pathFinderVersion)
		if err != nil {
			return 0, err
		}

		bucket := int(path[0])
		if bucketFiles[bucket] == nil {
			bucketFile, err := createRootHashBucketFile(bucketDir, bucket)
			if err != nil {
				return 0, err
			}
			bucketFiles[bucket] = bucketFile
		}

		if err := bucketFiles[bucket].write(path, leafHash); err != nil {
			return 0, fmt.Errorf("could not write register to bucket file of bucket %d: %w", bucket, err)
		}
		registerCount++
	}

	for _, bucketFile := range bucketFiles {
		if bucketFile == nil {
			continue
		}
		if err := bucketFile.Close(); err != nil {
			return 0, fmt.Errorf("could not close bucket file %s: %w", bucketFile.path, err)
		}
	}

	return registerCount, nil
}

// registerPathAndLeafHash returns the trie path and the leaf hash of the given register.
//
// No error returns are expected during normal operation.
func registerPathAndLeafHash(
	registerID flow.RegisterID,
	value flow.RegisterValue,
	pathFinderVersion uint8,
) (ledger.Path, hash.Hash, error) {
	path, err := pathfinder.KeyToPath(convert.RegisterIDToLedgerKey(registerID), pathFinderVersion)
	if err != nil {
		return ledger.DummyPath, hash.DummyHash, fmt.Errorf("could not compute path of register %s: %w", registerID, err)
	}

	// the leaf hash is the height-0 hash of the register's path and value; folding it up to a
	// node's height with the default hashes gives the hash of the compact leaf holding only
	// this register ([ledger.ComputeCompactValueFromLeafHash])
	return path, hash.HashLeaf(hash.Hash(path), value), nil
}

// rootHashBucketFile is the write end of a bucket file: a buffered file of records, each
// holding a register's path followed by the hash of its leaf.
type rootHashBucketFile struct {
	path   string
	file   *os.File
	writer *bufio.Writer
}

// createRootHashBucketFile creates the bucket file of the given bucket in the given directory.
//
// No error returns are expected during normal operation.
func createRootHashBucketFile(dir string, bucket int) (*rootHashBucketFile, error) {
	path := filepath.Join(dir, fmt.Sprintf("bucket-%03d", bucket))
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("could not create bucket file %s: %w", path, err)
	}

	return &rootHashBucketFile{
		path:   path,
		file:   file,
		writer: bufio.NewWriterSize(file, registerRootHashBucketBufSize),
	}, nil
}

// write appends the path and the leaf hash of a register to the bucket file.
//
// No error returns are expected during normal operation.
func (b *rootHashBucketFile) write(path ledger.Path, leafHash hash.Hash) error {
	if _, err := b.writer.Write(path[:]); err != nil {
		return err
	}
	_, err := b.writer.Write(leafHash[:])
	return err
}

// Close flushes and closes the bucket file. It is a no-op if the file is already closed.
//
// No error returns are expected during normal operation.
func (b *rootHashBucketFile) Close() error {
	if b.file == nil {
		return nil
	}
	flushErr := b.writer.Flush()
	closeErr := b.file.Close()
	b.file = nil
	return errors.Join(flushErr, closeErr)
}

// foldRegisterPathBuckets folds the register records of every bucket file into the hash of the
// trie node covering the bucket's paths, with up to workerCount buckets in parallel. Buckets
// without any register fold to the default hash of [registerRootHashBucketHeight].
//
// NOT CONCURRENCY SAFE!
//
// Expected error returns during normal operation:
//   - [context.Canceled], [context.DeadlineExceeded]: if the context is cancelled
func foldRegisterPathBuckets(ctx context.Context, bucketDir string, workerCount int) ([]hash.Hash, error) {
	subtreeHashes := make([]hash.Hash, registerRootHashBucketCount)
	for bucket := range subtreeHashes {
		subtreeHashes[bucket] = ledger.GetDefaultHashForHeight(registerRootHashBucketHeight)
	}

	g, _ := errgroup.WithContext(ctx)
	g.SetLimit(workerCount)
	for bucket := range registerRootHashBucketCount {
		bucketFile := filepath.Join(bucketDir, fmt.Sprintf("bucket-%03d", bucket))
		if _, err := os.Stat(bucketFile); err != nil {
			continue
		}

		g.Go(func() error {
			records, err := os.ReadFile(bucketFile)
			if err != nil {
				return fmt.Errorf("could not read bucket file %s: %w", bucketFile, err)
			}

			// every goroutine writes to its own bucket's hash
			subtreeHashes[bucket], err = foldRecords(records, registerRootHashBucketHeight)
			return err
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return subtreeHashes, nil
}

// foldRecords folds the register records of one bucket into the hash of the trie node at the
// given height covering all of them. The records hold the path and the leaf hash of each
// register, in no particular order.
//
// No error returns are expected during normal operation.
func foldRecords(records []byte, height int) (hash.Hash, error) {
	if len(records)%registerRootHashRecordLen != 0 {
		return hash.DummyHash, fmt.Errorf("bucket file has %d bytes, which is not a multiple of the record length %d",
			len(records), registerRootHashRecordLen)
	}

	count := len(records) / registerRootHashRecordLen
	if count == 0 {
		return hash.DummyHash, fmt.Errorf("bucket file does not hold any register")
	}

	// the records are not written to the bucket files in path order, so sort them by path; the
	// records stay in place and are sorted by index to keep the sorting cheap
	ordered := make([]uint32, count)
	for i := range ordered {
		ordered[i] = uint32(i)
	}
	slices.SortFunc(ordered, func(i, j uint32) int {
		return bytes.Compare(recordPath(records, i), recordPath(records, j))
	})

	return foldSortedRecords(records, ordered, 0, count, height)
}

// foldSortedRecords returns the hash of the trie node at the given height covering the records
// in ordered[from:to]. The records must be sorted by path and must all share the leading
// [ledger.NodeMaxHeight]-height bits of the node's sub tree.
//
// The hash is computed the way the ledger computes it:
//   - a node holding a single register is a compact leaf, whose hash is the register's leaf hash
//     extended to the node's height with default hashes ([ledger.ComputeCompactValueFromLeafHash])
//   - a node holding several registers hashes its two children, where a child without any
//     register hashes to the default hash of the child's height
//     ([ledger.GetDefaultHashForHeight])
//
// No error returns are expected during normal operation.
func foldSortedRecords(records []byte, ordered []uint32, from, to, height int) (hash.Hash, error) {
	if to-from == 1 {
		offset := int(ordered[from]) * registerRootHashRecordLen
		path := hash.Hash(records[offset : offset+hash.HashLen])
		leafHash := hash.Hash(records[offset+hash.HashLen : offset+registerRootHashRecordLen])
		return ledger.ComputeCompactValueFromLeafHash(path, leafHash, height), nil
	}

	if height == 0 {
		// the paths are distinct, so a sub tree of height 0 holds at most one register
		return hash.DummyHash, fmt.Errorf("bucket file holds %d registers with the same path", to-from)
	}

	// the children of the node are separated by the bit at the node's branch depth; the records
	// of the left child come before the records of the right child as they are sorted by path
	branchDepth := ledger.NodeMaxHeight - height
	split := from + sort.Search(to-from, func(i int) bool {
		path := recordPath(records, ordered[from+i])
		return bitutils.ReadBit(path, branchDepth) == 1
	})

	if split == from {
		// all registers are in the right child
		rightHash, err := foldSortedRecords(records, ordered, from, to, height-1)
		if err != nil {
			return hash.DummyHash, err
		}
		return hash.HashInterNode(ledger.GetDefaultHashForHeight(height-1), rightHash), nil
	}
	if split == to {
		// all registers are in the left child
		leftHash, err := foldSortedRecords(records, ordered, from, to, height-1)
		if err != nil {
			return hash.DummyHash, err
		}
		return hash.HashInterNode(leftHash, ledger.GetDefaultHashForHeight(height-1)), nil
	}

	leftHash, err := foldSortedRecords(records, ordered, from, split, height-1)
	if err != nil {
		return hash.DummyHash, err
	}
	rightHash, err := foldSortedRecords(records, ordered, split, to, height-1)
	if err != nil {
		return hash.DummyHash, err
	}
	return hash.HashInterNode(leftHash, rightHash), nil
}

// recordPath returns the path of the given record of a bucket file.
func recordPath(records []byte, record uint32) []byte {
	offset := int(record) * registerRootHashRecordLen
	return records[offset : offset+hash.HashLen]
}

// combineSubtreeRootHashes combines the subtree hashes of the buckets into the root hash of the
// whole trie, by hashing the pairs of neighbouring subtrees level by level.
func combineSubtreeRootHashes(subtreeHashes []hash.Hash) ledger.RootHash {
	// the buckets are ordered by the leading bits of their paths, so neighbouring buckets are
	// the children of the same node; combining them in place level by level yields the root
	for len(subtreeHashes) > 1 {
		for i := 0; i < len(subtreeHashes); i += 2 {
			subtreeHashes[i/2] = hash.HashInterNode(subtreeHashes[i], subtreeHashes[i+1])
		}
		subtreeHashes = subtreeHashes[:len(subtreeHashes)/2]
	}
	return ledger.RootHash(subtreeHashes[0])
}
