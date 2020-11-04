package jsonexporter

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger"
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

	w, err := wal.NewWAL(zerolog.Nop(), nil, ledgerPath, complete.DefaultCacheSize, pathfinder.PathByteSize, wal.SegmentSize)
	if err != nil {
		return fmt.Errorf("cannot create WAL: %w", err)
	}
	defer func() {
		_ = w.Close()
	}()

	// TODO port this to use new forest
	forest, err := mtrie.NewForest(pathfinder.PathByteSize, outputPath, complete.DefaultCacheSize, &metrics.NoopCollector{}, nil)
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
			err = forest.AddTries(rebuiltTries)
			if err != nil {
				return fmt.Errorf("adding rebuilt tries to forest failed: %w", err)
			}
			return nil
		},
		func(update *ledger.TrieUpdate) error {

			newTrieHash, err := forest.Update(update)

			for _, value := range update.Payloads {
				valuesSize += len(value.Value)
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

	log.Info().Int("values_count", valuesCount).Int("values_size_bytes", valuesSize).Int("updates_count", i).Float64("total_time_s", duration.Seconds()).Msg("finished seeking")
	log.Info().Msg("writing root checkpoint")

	trie, err := forest.GetTrie(targetHash)
	if err != nil {
		return fmt.Errorf("cannot get a trie with target hash: %w", err)
	}

	path := filepath.Join(outputPath, hex.EncodeToString(targetHash)+".trie.jsonl")

	fi, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fi.Close()

	writer := bufio.NewWriter(fi)
	defer writer.Flush()

	return trie.DumpAsJSON(writer)
}
