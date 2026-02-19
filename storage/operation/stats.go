package operation

import (
	"context"
	"encoding/json"
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
	Count       int     `json:"count"`
	MinSize     int     `json:"min_size"`
	MaxSize     int     `json:"max_size"`
	TotalSize   int     `json:"total_size"`
	AverageSize float64 `json:"avg_size"`
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start nWorker goroutines.
	for range nWorker {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return // Stop immediately on cancellation
				case prefix, ok := <-taskChan:
					if !ok {
						return // Stop if taskChan is closed
					}

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
			}
		}()
	}

	progress := util.LogProgress(log,
		util.DefaultLogProgressConfig(
			"Summarizing keys by first byte",
			256,
		))

	// Send all prefixes [0..255] to taskChan.
	for p := range 256 {
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

	var err error
	// If we encounter an error, we will return it immediately.
	for res := range resultChan {
		if res.err != nil {
			cancel() // Cancel running goroutines
			err = res.err
			break
		}
		finalStats[res.prefix] = res.stats
		log.Info().
			Int("prefix", int(res.prefix)).
			Int("total", res.stats.TotalSize).
			Int("count", res.stats.Count).
			Int("min", res.stats.MinSize).
			Int("max", res.stats.MaxSize).
			Msg("Processed prefix")
		progress(1) // log the progress
	}

	if err != nil {
		return nil, err
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
		err := item.Value(func(val []byte) error {
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
		})

		if err != nil {
			return s, fmt.Errorf("failed to process value for prefix %v: %w", int(prefix), err)
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
		return
	}

	// Convert map to a slice of key-value pairs
	statList := make([]struct {
		Prefix int   `json:"prefix"`
		Stats  Stats `json:"stats"`
	}, 0, len(stats))

	for p, s := range stats {
		statList = append(statList, struct {
			Prefix int   `json:"prefix"`
			Stats  Stats `json:"stats"`
		}{Prefix: int(p), Stats: s})
	}

	// Sort by TotalSize in ascending order
	sort.Slice(statList, func(i, j int) bool {
		return statList[i].Stats.TotalSize < statList[j].Stats.TotalSize
	})

	// Convert sorted stats to JSON
	jsonData, err := json.MarshalIndent(statList, "", "  ")
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal stats to JSON")
		return
	}

	// Log the JSON
	log.Info().RawJSON("stats", jsonData).Msg("Sorted prefix stats")
}
