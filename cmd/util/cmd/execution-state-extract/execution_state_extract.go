package extract

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/flattener"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/trie"
	"github.com/dapperlabs/flow-go/storage/ledger/wal"
)

func getStateCommitment(commits storage.Commits, blockHash flow.Identifier) (flow.StateCommitment, error) {
	return commits.ByBlockID(blockHash)
}

func extractExecutionState(dir string, targetHash flow.StateCommitment, outputDir string, log zerolog.Logger) error {

	w, err := wal.NewWAL(nil, nil, dir, 1000, 257)
	if err != nil {
		return fmt.Errorf("cannot create WAL: %w", err)
	}
	defer func() {
		_ = w.Close()
	}()

	mForest, err := mtrie.NewMForest(ledger.MaxHeight, outputDir, 1000, &metrics.NoopCollector{}, func(evictedTrie *trie.MTrie) error { return nil })
	if err != nil {
		return fmt.Errorf("cannot create mForest: %w", err)
	}

	i := 0

	valuesSize := 0
	valuesCount := 0
	startTime := time.Now()

	found := false

	FoundHashError := fmt.Errorf("found hash %s", targetHash)

	err = w.ReplayLogsOnly(func(stateCommitment flow.StateCommitment, keys [][]byte, values [][]byte) error {

		newTrie, err := mForest.Update(stateCommitment, keys, values)

		for _, value := range values {
			valuesSize += len(value)
		}

		valuesCount += len(values)

		if err != nil {
			return fmt.Errorf("error while updating mForest: %w", err)
		}

		if bytes.Equal(targetHash, newTrie.RootHash()) {
			found = true
			return FoundHashError
		}

		i++
		if i%1000 == 0 {
			log.Info().Int("values_count", valuesCount).Int("values_size_bytes", valuesSize).Int("updates_count", i).Msg("progress")
		}

		return err
	},
		func(commitment flow.StateCommitment) error {
			return nil
		})

	duration := time.Since(startTime)

	if !errors.Is(err, FoundHashError) {
		return fmt.Errorf("error while processing WAL: %w", err)
	}

	if !found {
		return fmt.Errorf("no value found: %w", err)
	}

	log.Info().Int("values_count", valuesCount).Int("values_size_bytes", valuesSize).Int("updates_count", i).Float64("total_time_s", duration.Seconds()).Msg("finished")

	flattenForest, err := flattener.FlattenForest(mForest)
	if err != nil {
		return fmt.Errorf("cannot flatten forest: %w", err)
	}

	checkpointWriter, err := wal.CreateCheckpointWriter(outputDir, 0)
	if err != nil {
		return fmt.Errorf("cannot create checkpointer writer: %w", err)
	}
	defer checkpointWriter.Close()

	err = wal.StoreCheckpoint(flattenForest, checkpointWriter)
	if err != nil {
		return fmt.Errorf("cannot store checkpoint: %w", err)
	}

	return nil
}
