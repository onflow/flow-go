package operation

import (
	"fmt"
	"math"
	"sort"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/storage"
)

// Stats holds statistics for a single prefix group.
type Stats struct {
	Count       int
	TotalSize   int
	MinSize     int
	MaxSize     int
	AverageSize float64
}

// SummarizeKeysByFirstByteConcurrent iterates over all prefixes [0x00..0xFF] in parallel
// using nWorker goroutines. Each worker handles one prefix at a time until all are processed.
//
// The storage.Reader must be able to create multiple iterators concurrently.
func SummarizeKeysByFirstByteConcurrent(log zerolog.Logger, r storage.Reader, nWorker int) (map[byte]Stats, error) {
	// We'll have at most 256 possible prefixes (0x00..0xFF).
	// Create tasks (one per prefix), a results channel, and a wait group.
	taskChan := make(chan byte, 256)
	resultChan := make(chan struct {
		prefix byte
		stats  Stats
		err    error
	}, 256)

	var wg sync.WaitGroup

	// Start nWorker goroutines.
	for i := 0; i < nWorker; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for prefix := range taskChan {
				st, err := processPrefix(r, prefix)
				resultChan <- struct {
					prefix byte
					stats  Stats
					err    error
				}{
					prefix: prefix,
					stats:  st,
					err:    err,
				}
			}
		}()
	}

	progress := util.LogProgress(log,
		util.DefaultLogProgressConfig(
			"Summarizing keys by first byte",
			256,
		))

	// Send all prefixes [0..255] to taskChan.
	for p := 0; p < 256; p++ {
		taskChan <- byte(p)
	}
	close(taskChan)

	// Once all workers finish, close the result channel.
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Gather results. We'll accumulate them in a map[prefix]Stats.
	finalStats := make(map[byte]Stats, 256)

	// If we encounter an error, we will return it immediately.
	for res := range resultChan {
		if res.err != nil {
			return nil, res.err
		}
		finalStats[res.prefix] = res.stats
		progress(1) // log the progress
	}

	return finalStats, nil
}

// processPrefix does the actual iteration and statistic calculation for a single prefix.
// It returns the Stats for that prefix, or an error if iteration fails.
func processPrefix(r storage.Reader, prefix byte) (Stats, error) {
	var s Stats
	// We use MinSize = math.MaxInt as a sentinel so the first real size will become the new minimum.
	s.MinSize = math.MaxInt

	// Iterator range is [prefix, prefix] (inclusive).
	start, end := []byte{prefix}, []byte{prefix}
	it, err := r.NewIter(start, end, storage.IteratorOption{BadgerIterateKeyOnly: true})
	if err != nil {
		return s, fmt.Errorf("failed to create iterator for prefix 0x%X: %w", prefix, err)
	}
	defer it.Close()

	for it.First(); it.Valid(); it.Next() {
		item := it.IterItem()

		// item.Value(...) is a function call that gives us the value, on which we measure size.
		if err := item.Value(func(val []byte) error {
			size := len(val)
			s.Count++
			s.TotalSize += size
			if size < s.MinSize {
				s.MinSize = size
			}
			if size > s.MaxSize {
				s.MaxSize = size
			}
			return nil
		}); err != nil {
			return s, fmt.Errorf("failed to process value for prefix 0x%X: %w", prefix, err)
		}
	}

	// If we found no keys for this prefix, reset MinSize to 0 to avoid confusion.
	if s.Count == 0 {
		s.MinSize = 0
	} else {
		// Compute average size.
		s.AverageSize = float64(s.TotalSize) / float64(s.Count)
	}

	return s, nil
}

// PrintStats logs the statistics for each prefix in ascending order.
// Each prefix is shown in hex, along with count, min, max, total, and average sizes.
func PrintStats(log zerolog.Logger, stats map[byte]Stats) {
	if len(stats) == 0 {
		log.Info().Msg("No stats to print (map is empty).")
	}

	// Sort the prefix keys so logs appear in ascending order.
	prefixes := make([]int, 0, len(stats))
	for p := range stats {
		prefixes = append(prefixes, int(p))
	}
	sort.Ints(prefixes)

	// Print each prefix's stats.
	for _, p := range prefixes {
		s := stats[byte(p)]
		// Format the prefix as 0xNN
		prefixLabel := fmt.Sprintf("0x%02X", p)

		log.Info().
			Str("prefix", prefixLabel).
			Int("count", s.Count).
			Int("min_size", s.MinSize).
			Int("max_size", s.MaxSize).
			Int("total_size", s.TotalSize).
			Float64("avg_size", s.AverageSize).
			Msg("Prefix stats")
	}
}
