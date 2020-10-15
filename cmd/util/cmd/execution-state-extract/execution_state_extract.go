package extract

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path"
	"time"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/storage/badger/operation"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	oldLedger "github.com/onflow/flow-go/storage/ledger"
	oldMtrie "github.com/onflow/flow-go/storage/ledger/mtrie"
	oldFlattener "github.com/onflow/flow-go/storage/ledger/mtrie/flattener"
	"github.com/onflow/flow-go/storage/ledger/mtrie/trie"
	oldWal "github.com/onflow/flow-go/storage/ledger/wal"
)

//
func getStateCommitment(commits storage.Commits, blockHash flow.Identifier) (flow.StateCommitment, error) {
	return commits.ByBlockID(blockHash)
}

func getMappingsFromDatabase(db *badger.DB) (map[string]delta.Mapping, error) {

	mappings := make(map[string]delta.Mapping)

	var found [][]*delta.LegacySnapshot
	err := db.View(operation.FindLegacyExecutionStateInteractions(func(interactions []*delta.LegacySnapshot) bool {

		for _, interaction := range interactions {
			for k, mapping := range interaction.Delta.ReadMappings {
				mappings[k] = mapping
			}
			for k, mapping := range interaction.Delta.WriteMappings {
				mappings[k] = mapping
			}
		}

		return false
	}, &found))

	return mappings, err
}

func ReadMegamappings(filename string) (map[string]delta.Mapping, error) {
	var readMappings = map[string]delta.Mapping{}
	var hexencodedRead map[string]delta.Mapping

	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("cannot open mappings file: %w", err)
	}
	err = json.Unmarshal(bytes, &hexencodedRead)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshall mappings: %w", err)
	}

	for k, mapping := range hexencodedRead {
		decodeString, err := hex.DecodeString(k)
		if err != nil {
			return nil, fmt.Errorf("cannot decode key: %w", err)
		}
		readMappings[string(decodeString)] = mapping
	}

	return readMappings, nil
}

//
func extractExecutionState(dir string, targetHash flow.StateCommitment, outputDir string, log zerolog.Logger, mappings map[string]delta.Mapping) error {

	w, err := oldWal.NewWAL(nil, nil, dir, oldLedger.CacheSize, oldLedger.RegisterKeySize, oldWal.SegmentSize)
	if err != nil {
		return fmt.Errorf("cannot create WAL: %w", err)
	}
	defer func() {
		_ = w.Close()
	}()

	oldMForest, err := oldMtrie.NewMForest(oldLedger.RegisterKeySize, outputDir, 1000, &metrics.NoopCollector{}, func(evictedTrie *trie.MTrie) error { return nil })
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
		func(forestSequencing *oldFlattener.FlattenedForest) error {
			rebuiltTries, err := oldFlattener.RebuildTries(forestSequencing)
			if err != nil {
				return fmt.Errorf("rebuilding forest from sequenced nodes failed: %w", err)
			}
			err = oldMForest.AddTries(rebuiltTries)
			if err != nil {
				return fmt.Errorf("adding rebuilt tries to forest failed: %w", err)
			}
			return nil
		},
		func(stateCommitment flow.StateCommitment, keys [][]byte, values [][]byte) error {

			newTrie, err := oldMForest.Update(stateCommitment, keys, values)

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

	log.Info().Int("values_count", valuesCount).Int("values_size_bytes", valuesSize).Int("updates_count", i).Float64("total_time_s", duration.Seconds()).Msg("finished replaying")

	//remove other tries
	tries, err := oldMForest.GetTries()
	if err != nil {
		return fmt.Errorf("cannot get tries: %w", err)
	}

	for _, mTrie := range tries {
		if !bytes.Equal(mTrie.RootHash(), targetHash) {
			oldMForest.RemoveTrie(mTrie.RootHash())
		}
	}

	// check if we have only one trie
	tries, err = oldMForest.GetTries()
	if err != nil {
		return fmt.Errorf("cannot get tries again: %w", err)
	}

	if len(tries) != 1 {
		return fmt.Errorf("too many tries left after filtering: %w", err)
	}

	// converting data
	log.Info().Msg("Converting data")
	rootTrie := tries[0]

	iterator := oldFlattener.NewNodeIterator(rootTrie)

	forest, err := mtrie.NewForest(pathfinder.PathByteSize, "", 1000, &metrics.NoopCollector{}, nil)
	if err != nil {
		return fmt.Errorf("cannot create new forest: %w", err)
	}

	paths := make([]ledger.Path, 0)
	payloads := make([]*ledger.Payload, 0)

	for iterator.Next() {
		node := iterator.Value()

		if !node.IsLeaf() {
			continue
		}

		oldKey := node.Key()
		oldValue := node.Value()

		mapping, ok := mappings[string(oldKey)]
		if !ok {
			return fmt.Errorf("mapping not found for key %x", oldKey)
		}

		registerID := flow.NewRegisterID(mapping.Owner, mapping.Controller, mapping.Key)
		key := state.RegisterIDToKey(registerID)

		path, err := pathfinder.KeyToPath(key, 0)
		if err != nil {
			return fmt.Errorf("cannot convert key to path: %w", err)
		}

		paths = append(paths, path)
		payload := ledger.NewPayload(key, oldValue)
		payloads = append(payloads, payload)
	}

	update := &ledger.TrieUpdate{
		RootHash: forest.GetEmptyRootHash(),
		Paths:    paths,
		Payloads: payloads,
	}

	newHash, err := forest.Update(update)
	if err != nil {
		return fmt.Errorf("cannot update trie with new values: %w", err)
	}

	log.Info().Msgf("NEW STATE COMMITMENT: %s", hex.EncodeToString(newHash))

	log.Info().Msg("writing root checkpoint")

	startTime = time.Now()

	flattenForest, err := flattener.FlattenForest(forest)
	if err != nil {
		return fmt.Errorf("cannot flatten forest: %w", err)
	}

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
