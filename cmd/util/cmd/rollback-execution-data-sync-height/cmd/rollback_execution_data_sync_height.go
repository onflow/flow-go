package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/ipfs/go-cid"
	pebbleds "github.com/ipfs/go-ds-pebble"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store"
)

var (
	flagHeight           uint64
	flagExecutionDataDir string
)

var Cmd = &cobra.Command{
	Use:   "rollback-execution-data-sync-height",
	Short: "Rollback the execution data sync height",
	RunE:  runE,
}

func init() {
	// execution data from height + 1 will be removed
	Cmd.Flags().Uint64Var(&flagHeight, "height", 0,
		"the height of the block to rollback execution data sync to")
	_ = Cmd.MarkFlagRequired("height")

	Cmd.Flags().StringVar(&flagExecutionDataDir, "execution-data-dir", "/var/flow/data/execution_data",
		"directory that stores the execution data blobstore")
	_ = Cmd.MarkFlagRequired("execution-data-dir")
}

func runE(*cobra.Command, []string) error {
	lockManager := storage.MakeSingletonLockManager()

	log.Info().
		Str("datadir", flagDatadir).
		Str("execution-data-dir", flagExecutionDataDir).
		Uint64("height", flagHeight).
		Msg("flags")

	if flagHeight == 0 {
		return fmt.Errorf("height must be above 0: %v", flagHeight)
	}

	return common.WithStorage(flagDatadir, func(db storage.DB) error {
		storages := common.InitStorages(db)
		state, err := common.OpenProtocolState(lockManager, db, storages)
		if err != nil {
			return fmt.Errorf("could not open protocol states: %w", err)
		}

		// Get the current progress heights and validate rollback height
		sealedRootHeight := state.Params().SealedRoot().Height

		// Validate that rollback height is not lower than SealedRoot height
		if flagHeight < sealedRootHeight {
			return fmt.Errorf("cannot rollback to height %v: it is lower than SealedRoot height %v",
				flagHeight, sealedRootHeight)
		}

		requesterProgress, err := store.NewConsumerProgress(db, module.ConsumeProgressExecutionDataRequesterBlockHeight).Initialize(sealedRootHeight)
		if err != nil {
			return fmt.Errorf("could not initialize execution data requester progress: %w", err)
		}

		requesterHeight, err := requesterProgress.ProcessedIndex()
		if err != nil {
			return fmt.Errorf("could not get execution data requester height: %w", err)
		}

		// Validate that rollback height is not beyond the requester progress height
		// Note: We only rollback the requester progress, not the indexer progress
		if flagHeight > requesterHeight {
			return fmt.Errorf("cannot rollback to height %v: it is beyond the requester progress height %v",
				flagHeight, requesterHeight)
		}

		metrics := &metrics.NoopCollector{}
		seals := store.NewSeals(metrics, db)
		results := store.NewExecutionResults(metrics, db)

		// Initialize execution data blobstore
		executionDataBlobstore, err := initExecutionDataBlobstore(flagExecutionDataDir)
		if err != nil {
			return fmt.Errorf("could not initialize execution data blobstore: %w", err)
		}

		// remove execution data from the specified height
		// Note: rollback height is the highest height that is NOT removed (we start removing from rollback height + 1)
		removeFromHeight := flagHeight + 1
		err = removeExecutionDataFromHeight(
			context.Background(),
			state,
			seals,
			results,
			executionDataBlobstore,
			removeFromHeight)

		if err != nil {
			return fmt.Errorf("could not remove execution data from height %v: %w", removeFromHeight, err)
		}

		// Reset only the requester progress height to the rollback height
		// Note: The indexer progress and indexed data (events, collections, transactions, results, registers) are NOT rolled back
		protocolDBBatch := db.NewBatch()
		defer protocolDBBatch.Close()

		err = requesterProgress.BatchSetProcessedIndex(flagHeight, protocolDBBatch)
		if err != nil {
			return fmt.Errorf("could not set execution data requester height to %v: %w", flagHeight, err)
		}

		err = protocolDBBatch.Commit()
		if err != nil {
			return fmt.Errorf("could not flush write batch at %v: %w", flagHeight, err)
		}

		log.Info().
			Uint64("rollback_height", flagHeight).
			Uint64("previous_requester_height", requesterHeight).
			Msg("execution data requester height rolled back (indexer progress and indexed data preserved)")

		return nil
	})
}

// removeExecutionDataFromHeight removes all execution data from the specified block height onward
// to the latest finalized height.
func removeExecutionDataFromHeight(
	ctx context.Context,
	protoState protocol.State,
	seals storage.Seals,
	results storage.ExecutionResults,
	blobstore blobs.Blobstore,
	fromHeight uint64,
) error {
	log.Info().Msgf("removing execution data for blocks from height: %v", fromHeight)

	root := protoState.Params().FinalizedRoot()

	if fromHeight <= root.Height {
		return fmt.Errorf("can only remove execution data for blocks above root block. fromHeight: %v, rootHeight: %v", fromHeight, root.Height)
	}

	final, err := protoState.Final().Head()
	if err != nil {
		return fmt.Errorf("could not get finalized height: %w", err)
	}

	if fromHeight > final.Height {
		return fmt.Errorf("could not remove execution data for unfinalized height: %v, finalized height: %v", fromHeight, final.Height)
	}

	finalRemoved := 0
	total := int(final.Height-fromHeight) + 1

	// removing for finalized blocks from highest to lowest
	for height := final.Height; height >= fromHeight; height-- {
		head, err := protoState.AtHeight(height).Head()
		if err != nil {
			return fmt.Errorf("could not get header at height: %w", err)
		}

		blockID := head.ID()

		err = removeExecutionDataForBlock(ctx, blockID, seals, results, blobstore)
		if err != nil {
			return fmt.Errorf("could not remove execution data for finalized block: %v, %w", blockID, err)
		}

		finalRemoved++
		log.Info().Msgf("execution data at height %v has been removed. progress (%v/%v)", height, finalRemoved, total)
	}

	log.Info().Msgf("removed execution data from height %v. removed for %v finalized blocks",
		fromHeight, finalRemoved)

	return nil
}

// removeExecutionDataForBlock removes all execution data blobs for a given block.
// This removes data inserted by ExecutionDataStore.Add.
func removeExecutionDataForBlock(
	ctx context.Context,
	blockID flow.Identifier,
	seals storage.Seals,
	results storage.ExecutionResults,
	blobstore blobs.Blobstore,
) error {
	// Get seal by block ID
	seal, err := seals.FinalizedSealForBlock(blockID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			log.Info().Msgf("seal not found for block %v", blockID)
			return nil
		}
		return fmt.Errorf("could not find seal for block %v: %w", blockID, err)
	}

	// Get execution result by result ID from the seal
	result, err := results.ByID(seal.ResultID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			log.Info().Msgf("execution result not found for result ID %v (block %v)", seal.ResultID, blockID)
			return nil
		}
		return fmt.Errorf("could not find execution result for result ID %v (block %v): %w", seal.ResultID, blockID, err)
	}

	// Check if execution data ID exists
	if result.ExecutionDataID == flow.ZeroID {
		log.Info().Msgf("execution data ID is zero for block %v", blockID)
		return nil
	}

	// Collect all CIDs from the execution data tree
	allCIDs, err := collectAllCIDsFromExecutionData(ctx, blobstore, result.ExecutionDataID)
	if err != nil {
		return fmt.Errorf("could not collect CIDs from execution data for block %v: %w", blockID, err)
	}

	// Delete all blobs
	for _, cid := range allCIDs {
		err = blobstore.DeleteBlob(ctx, cid)
		if err != nil {
			// Continue even if blob is not found (may have been already deleted)
			if !errors.Is(err, blobs.ErrNotFound) {
				return fmt.Errorf("could not delete blob %v: %w", cid, err)
			}
		}
	}

	log.Info().Msgf("removed %v blobs for execution data of block %v", len(allCIDs), blockID)

	return nil
}

// collectAllCIDsFromExecutionData collects all CIDs from the execution data blob tree.
func collectAllCIDsFromExecutionData(
	ctx context.Context,
	blobstore blobs.Blobstore,
	rootID flow.Identifier,
) ([]cid.Cid, error) {
	allCIDs := make(map[cid.Cid]struct{})
	rootCid := flow.IdToCid(rootID)
	allCIDs[rootCid] = struct{}{}

	// Get the root blob to find chunk execution data CIDs
	rootBlob, err := blobstore.Get(ctx, rootCid)
	if err != nil {
		if errors.Is(err, blobs.ErrNotFound) {
			// Already deleted, return empty
			return []cid.Cid{}, nil
		}
		return nil, fmt.Errorf("could not get root blob: %w", err)
	}

	// Deserialize to get BlockExecutionDataRoot
	rootData, err := execution_data.DefaultSerializer.Deserialize(bytes.NewBuffer(rootBlob.RawData()))
	if err != nil {
		return nil, fmt.Errorf("could not deserialize root blob: %w", err)
	}

	executionDataRoot, ok := rootData.(*flow.BlockExecutionDataRoot)
	if !ok {
		return nil, fmt.Errorf("root blob does not deserialize to BlockExecutionDataRoot")
	}

	// Collect CIDs from each chunk execution data tree
	for _, chunkCID := range executionDataRoot.ChunkExecutionDataIDs {
		chunkCIDs, err := collectCIDsFromChunkExecutionData(ctx, blobstore, chunkCID)
		if err != nil {
			return nil, fmt.Errorf("could not collect CIDs from chunk execution data: %w", err)
		}
		for _, cid := range chunkCIDs {
			allCIDs[cid] = struct{}{}
		}
	}

	// Convert map to slice
	result := make([]cid.Cid, 0, len(allCIDs))
	for cid := range allCIDs {
		result = append(result, cid)
	}

	return result, nil
}

// collectCIDsFromChunkExecutionData recursively collects all CIDs from a chunk execution data blob tree.
func collectCIDsFromChunkExecutionData(
	ctx context.Context,
	blobstore blobs.Blobstore,
	chunkCID cid.Cid,
) ([]cid.Cid, error) {
	allCIDs := make(map[cid.Cid]struct{})
	toProcess := []cid.Cid{chunkCID}

	for len(toProcess) > 0 {
		currentCID := toProcess[0]
		toProcess = toProcess[1:]

		if _, seen := allCIDs[currentCID]; seen {
			continue
		}
		allCIDs[currentCID] = struct{}{}

		// Get the blob
		blob, err := blobstore.Get(ctx, currentCID)
		if err != nil {
			if errors.Is(err, blobs.ErrNotFound) {
				continue // Skip if already deleted
			}
			return nil, fmt.Errorf("could not get blob %v: %w", currentCID, err)
		}

		// Try to deserialize as a list of CIDs (intermediate level)
		data, err := execution_data.DefaultSerializer.Deserialize(bytes.NewBuffer(blob.RawData()))
		if err != nil {
			// If deserialization fails, it's a leaf blob (actual data), so we're done with this branch
			continue
		}

		// Check if it's a list of CIDs
		if cidList, ok := data.(*[]cid.Cid); ok {
			// Add child CIDs to process
			for _, childCID := range *cidList {
				toProcess = append(toProcess, childCID)
			}
		}
		// If it's not a list of CIDs, it's a leaf (ChunkExecutionData), so we're done
	}

	result := make([]cid.Cid, 0, len(allCIDs))
	for cid := range allCIDs {
		result = append(result, cid)
	}

	return result, nil
}

// initExecutionDataBlobstore initializes the execution data blobstore from the given directory.
func initExecutionDataBlobstore(executionDataDir string) (blobs.Blobstore, error) {
	datastoreDir := filepath.Join(executionDataDir, "blobstore")
	ds, err := pebbleds.NewDatastore(datastoreDir, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create pebble datastore at %v: %w", datastoreDir, err)
	}

	return blobs.NewBlobstore(ds), nil
}
