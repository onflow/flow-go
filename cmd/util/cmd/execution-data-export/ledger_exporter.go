package exporter

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/dapperlabs/flow-go/cmd/util/cmd/common"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/flattener"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/trie"
	"github.com/dapperlabs/flow-go/storage/ledger/wal"
	"github.com/rs/zerolog/log"
)

// ExportLedger exports ledger key value pairs at the given blockID
func ExportLedger(blockID flow.Identifier, dbPath string, ledgerPath string, outputPath string) error {
	db := common.InitStorage(dbPath)
	defer db.Close()

	cache := &metrics.NoopCollector{}
	commits := badger.NewCommits(cache, db)

	targetHash, err := commits.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("cannot get state commitment for block: %w", err)
	}

	w, err := wal.NewWAL(nil, nil, ledgerPath, ledger.CacheSize, ledger.RegisterKeySize, wal.SegmentSize)
	if err != nil {
		return fmt.Errorf("cannot create WAL: %w", err)
	}
	defer func() {
		_ = w.Close()
	}()

	// TODO port this to use new forest
	mForest, err := mtrie.NewMForest(ledger.RegisterKeySize, outputPath, 1000, &metrics.NoopCollector{}, func(evictedTrie *trie.MTrie) error { return nil })
	if err != nil {
		return fmt.Errorf("cannot create mForest: %w", err)
	}

	i := 0
	valuesSize := 0
	valuesCount := 0
	startTime := time.Now()
	found := false
	FoundHashError := fmt.Errorf("found hash %s", targetHash)

	err = w.ReplayLogsOnly(
		func(forestSequencing *flattener.FlattenedForest) error {
			rebuiltTries, err := flattener.RebuildTries(forestSequencing)
			if err != nil {
				return fmt.Errorf("rebuilding forest from sequenced nodes failed: %w", err)
			}
			err = mForest.AddTries(rebuiltTries)
			if err != nil {
				return fmt.Errorf("adding rebuilt tries to forest failed: %w", err)
			}
			return nil
		},
		func(stateCommitment flow.StateCommitment, keys [][]byte, values [][]byte) error {

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

	log.Info().Int("values_count", valuesCount).Int("values_size_bytes", valuesSize).Int("updates_count", i).Float64("total_time_s", duration.Seconds()).Msg("finished seeking")
	log.Info().Msg("writing root checkpoint")

	mForest.DumpTrieAsJSON(targetHash, filepath.Join(outputPath, hex.EncodeToString(targetHash)+".trie.jsonl"))
	return nil
}
