package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ipfs/go-cid"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	edstorage "github.com/onflow/flow-go/module/executiondatasync/storage"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store"
)

var (
	flagHeight           uint64
	flagExecutionDataDir string
	flagWorkerCount      uint
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

	Cmd.Flags().UintVar(&flagWorkerCount, "worker-count", 1,
		"number of workers to use for concurrent processing, default is 1")
}

func runE(*cobra.Command, []string) error {
	lockManager := storage.MakeSingletonLockManager()

	log.Info().
		Str("datadir", flagDatadir).
		Str("execution-data-dir", flagExecutionDataDir).
		Uint64("height", flagHeight).
		Uint("worker-count", flagWorkerCount).
		Msg("flags")

	if flagWorkerCount < 1 {
		return fmt.Errorf("worker count must be at least 1, but got %v", flagWorkerCount)
	}

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

		// Initialize execution data datastore manager to access the execution data store DB
		// Note: ConsumeProgressExecutionDataRequesterBlockHeight is stored in the execution data
		// datastore DB, not the protocol DB, since that is where the jobqueue writes execution data to.
		executionDatastoreManager, err := edstorage.CreateDatastoreManager(
			log.Logger, flagExecutionDataDir)
		if err != nil {
			return fmt.Errorf("could not create execution data datastore manager: %w", err)
		}
		defer func() {
			if closeErr := executionDatastoreManager.Close(); closeErr != nil {
				log.Error().Err(closeErr).Msg("failed to close execution data datastore manager")
			}
		}()

		// Use the execution data datastore DB instead of the protocol DB
		executionDataDB := executionDatastoreManager.DB()

		requesterProgress, err := store.NewConsumerProgress(executionDataDB, module.ConsumeProgressExecutionDataRequesterBlockHeight).Initialize(sealedRootHeight)
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

		// Create execution data blobstore from the datastore manager
		executionDataBlobstore := blobs.NewBlobstore(executionDatastoreManager.Datastore())

		// remove execution data from the specified height
		// Note: rollback height is the highest height that is NOT removed (we start removing from rollback height + 1)
		removeFromHeight := flagHeight + 1
		err = removeExecutionDataFromHeight(
			context.Background(),
			state,
			seals,
			results,
			executionDataBlobstore,
			removeFromHeight,
			flagWorkerCount)

		if err != nil {
			return fmt.Errorf("could not remove execution data from height %v: %w", removeFromHeight, err)
		}

		// Reset only the requester progress height to the rollback height
		// Note: The indexer progress and indexed data (events, collections, transactions, results, registers) are NOT rolled back
		err = executionDataDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return requesterProgress.BatchSetProcessedIndex(flagHeight, rw)
		})
		if err != nil {
			return fmt.Errorf("could not set execution data requester height to %v: %w", flagHeight, err)
		}

		log.Info().
			Uint64("rollback_height", flagHeight).
			Uint64("previous_requester_height", requesterHeight).
			Msg("execution data requester height rolled back (indexer progress and indexed data preserved)")

		return nil
	})
}

// removeExecutionDataFromHeight removes all execution data from the specified block height onward
// to the latest finalized height using concurrent workers.
func removeExecutionDataFromHeight(
	ctx context.Context,
	protoState protocol.State,
	seals storage.Seals,
	results storage.ExecutionResults,
	blobstore blobs.Blobstore,
	fromHeight uint64,
	workerCount uint,
) error {
	log.Info().
		Uint64("from-height", fromHeight).
		Uint("worker-count", workerCount).
		Msg("removing execution data for blocks")

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

	total := int(final.Height-fromHeight) + 1
	if total <= 0 {
		log.Info().Msg("no blocks to remove")
		return nil
	}

	// Create channel for heights to process
	heights := make(chan uint64, int(workerCount))
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Shared state for progress tracking and error handling
	var (
		mu           sync.Mutex
		removedCount int
		firstErr     error
	)

	// Worker function
	worker := func() {
		for {
			select {
			case <-workCtx.Done():
				return
			case height, ok := <-heights:
				if !ok {
					return
				}

				head, err := protoState.AtHeight(height).Head()
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = fmt.Errorf("could not get header at height %v: %w", height, err)
						cancel()
					}
					mu.Unlock()
					return
				}

				blockID := head.ID()
				err = removeExecutionDataForBlock(workCtx, blockID, seals, results, blobstore)
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = fmt.Errorf("could not remove execution data for finalized block %v at height %v: %w", blockID, height, err)
						cancel()
					}
					mu.Unlock()
					return
				}

				mu.Lock()
				removedCount++
				currentProgress := removedCount
				mu.Unlock()

				log.Info().
					Uint64("height", height).
					Int("progress", currentProgress).
					Int("total", total).
					Msg("execution data at height has been removed")
			}
		}
	}

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < int(workerCount); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker()
		}()
	}

	// Send heights to workers (from highest to lowest)
	go func() {
		defer close(heights)
		for height := final.Height; height >= fromHeight; height-- {
			select {
			case <-workCtx.Done():
				return
			case heights <- height:
			}
		}
	}()

	// Wait for all workers to complete
	wg.Wait()

	if firstErr != nil {
		return firstErr
	}

	log.Info().
		Uint64("from-height", fromHeight).
		Int("removed-blocks", removedCount).
		Msg("removed execution data from height")

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
