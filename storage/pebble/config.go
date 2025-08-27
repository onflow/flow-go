package pebble

import (
	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/bloom"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/storage/util"
)

// DefaultPebbleOptions returns an optimized set of pebble options.
// This is mostly copied form pebble's nightly performance benchmark.
func DefaultPebbleOptions(logger zerolog.Logger, cache *pebble.Cache, comparer *pebble.Comparer) *pebble.Options {
	opts := &pebble.Options{
		Cache:              cache,
		Comparer:           comparer,
		FormatMajorVersion: pebble.FormatVirtualSSTables,

		// Soft and hard limits on read amplificaction of L0 respectfully.
		L0CompactionThreshold: 2,
		L0StopWritesThreshold: 1000,

		// When the maximum number of bytes for a level is exceeded, compaction is requested.
		LBaseMaxBytes: 64 << 20, // 64 MB
		Levels:        make([]pebble.LevelOptions, 7),
		MaxOpenFiles:  16384,

		// Writes are stopped when the sum of the queued memtable sizes exceeds MemTableStopWritesThreshold*MemTableSize.
		MemTableSize:                64 << 20,
		MemTableStopWritesThreshold: 4,

		// The default is 1.
		MaxConcurrentCompactions: func() int { return 4 },
		Logger:                   util.NewLogger(logger),
	}

	for i := 0; i < len(opts.Levels); i++ {
		l := &opts.Levels[i]
		// The default is 4KiB (uncompressed), which is too small
		// for good performance (esp. on stripped storage).
		l.BlockSize = 32 << 10       // 32 KB
		l.IndexBlockSize = 256 << 10 // 256 KB

		// The bloom filter speedsup our SeekPrefixGE by skipping
		// sstables that do not contain the prefix
		l.FilterPolicy = bloom.FilterPolicy(MinLookupKeyLen)
		l.FilterType = pebble.TableFilter

		if i > 0 {
			// L0 starts at 2MiB, each level is 2x the previous.
			l.TargetFileSize = opts.Levels[i-1].TargetFileSize * 2
		}
		l.EnsureDefaults()
	}

	// TODO(rbtz): benchmark with and without bloom filters on L6
	// opts.Levels[6].FilterPolicy = nil

	// Splitting sstables during flush allows increased compaction flexibility and concurrency when those
	// tables are compacted to lower levels.
	opts.FlushSplitBytes = opts.Levels[0].TargetFileSize
	opts.EnsureDefaults()

	return opts
}
