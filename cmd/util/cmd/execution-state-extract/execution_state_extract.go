package extract

import (
	"bytes"
	"errors"
	"fmt"
	"path"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/wal"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
)

func getStateCommitment(commits storage.Commits, blockHash flow.Identifier) (flow.StateCommitment, error) {
	return commits.ByBlockID(blockHash)
}

func extractExecutionState(dir string, targetHash flow.StateCommitment, outputDir string, log zerolog.Logger) error {

	w, err := wal.NewWAL(nil, nil, dir, complete.DefaultCacheSize, pathfinder.PathByteSize, wal.SegmentSize)
	if err != nil {
		return fmt.Errorf("cannot create WAL: %w", err)
	}
	defer func() {
		_ = w.Close()
	}()

	forest, err := mtrie.NewForest(pathfinder.PathByteSize, outputDir, complete.DefaultCacheSize, &metrics.NoopCollector{}, func(evictedTrie *trie.MTrie) error { return nil })
	if err != nil {
		return fmt.Errorf("cannot create mForest: %w", err)
	}

	i := 0

	valuesSize := 0
	valuesCount := 0
	startTime := time.Now()

	found := false

	FoundHashError := fmt.Errorf("found hash %s", targetHash)

	log.Info().Msg("Replaying WAL")

	err = w.ReplayLogsOnly(
		func(forestSequencing *flattener.FlattenedForest) error {
			rebuiltTries, err := flattener.RebuildTries(forestSequencing)
			if err != nil {
				return fmt.Errorf("rebuilding forest from sequenced nodes failed: %w", err)
			}
			err = forest.AddTries(rebuiltTries)
			if err != nil {
				return fmt.Errorf("adding rebuilt tries to forest failed: %w", err)
			}
			return nil
		},
		func(update *ledger.TrieUpdate) error {

			newTrieHash, err := forest.Update(update)

			for _, payload := range update.Payloads {
				valuesSize += len(payload.Value)
			}

			valuesCount += len(update.Payloads)

			if err != nil {
				return fmt.Errorf("error while updating mForest: %w", err)
			}

			if bytes.Equal(targetHash, newTrieHash) {
				found = true
				return FoundHashError
			}

			i++
			if i%1000 == 0 {
				log.Info().Int("values_count", valuesCount).Int("values_size_bytes", valuesSize).Int("updates_count", i).Msg("progress")
			}

			return err
		},
		func(commitment ledger.RootHash) error {
			return nil
		})

	duration := time.Since(startTime)

	if !errors.Is(err, FoundHashError) {
		return fmt.Errorf("error while processing WAL: %w", err)
	}

	if !found {
		return fmt.Errorf("no value found: %w", err)
	}

	log.Info().Int("values_count", valuesCount).Int("values_size_bytes", valuesSize).Int("updates_count", i).Float64("total_time_s", duration.Seconds()).Msg("finished replaying")

	//remove other tries
	tries, err := forest.GetTries()
	if err != nil {
		return fmt.Errorf("cannot get tries: %w", err)
	}

	for _, mTrie := range tries {
		if !bytes.Equal(mTrie.RootHash(), targetHash) {
			forest.RemoveTrie(mTrie.RootHash())
		}
	}

	// check if we have only one trie
	tries, err = forest.GetTries()
	if err != nil {
		return fmt.Errorf("cannot get tries again: %w", err)
	}

	if len(tries) != 1 {
		return fmt.Errorf("too many tries left after filtering: %w", err)
	}

	log.Info().Msg("writing root checkpoint")

	startTime = time.Now()

	flattenForest, err := flattener.FlattenForest(forest)
	if err != nil {
		return fmt.Errorf("cannot flatten forest: %w", err)
	}

	// migrations

	checkpointWriter, err := wal.CreateCheckpointWriterForFile(path.Join(outputDir, wal.RootCheckpointFilename))
	if err != nil {
		return fmt.Errorf("cannot create checkpointer writer: %w", err)
	}
	defer func() {
		err := checkpointWriter.Close()
		if err != nil {
			log.Err(err).Msg("error while writing checkpoint")
		}
	}()

	err = wal.StoreCheckpoint(flattenForest, checkpointWriter)
	if err != nil {
		return fmt.Errorf("cannot store checkpoint: %w", err)
	}

	duration = time.Since(startTime)
	log.Info().Float64("total_time_s", duration.Seconds()).Msg("finished writing checkpoiunt")

	return nil
}
