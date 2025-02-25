package verifier

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/initialize"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification/convert"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/chunks"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	storagepebble "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/storage/store"
)

// VerifyLastKHeight verifies the last k sealed blocks by verifying all chunks in the results.
// It assumes the latest sealed block has been executed, and the chunk data packs have not been
// pruned.
// Note, it returns nil if certain block is not executed, in this case warning will be logged
func VerifyLastKHeight(k uint64, chainID flow.ChainID, protocolDataDir string, chunkDataPackDir string, nWorker uint, stopOnMismatch bool) (err error) {
	closer, storages, chunkDataPacks, state, verifier, err := initStorages(chainID, protocolDataDir, chunkDataPackDir)
	if err != nil {
		return fmt.Errorf("could not init storages: %w", err)
	}
	defer func() {
		closerErr := closer()
		if closerErr != nil {
			err = errors.Join(err, closerErr)
		}
	}()

	lastSealed, err := state.Sealed().Head()
	if err != nil {
		return fmt.Errorf("could not get last sealed height: %w", err)
	}

	root := state.Params().SealedRoot().Height

	// preventing overflow
	if k > lastSealed.Height+1 {
		return fmt.Errorf("k is greater than the number of sealed blocks, k: %d, last sealed height: %d", k, lastSealed.Height)
	}

	from := lastSealed.Height - k + 1

	// root block is not verifiable, because it's sealed already.
	// the first verifiable is the next block of the root block
	firstVerifiable := root + 1

	if from < firstVerifiable {
		from = firstVerifiable
	}
	to := lastSealed.Height

	log.Info().Msgf("verifying blocks from %d to %d", from, to)

	err = verifyConcurrently(from, to, nWorker, stopOnMismatch, storages.Headers, chunkDataPacks, storages.Results, state, verifier, verifyHeight)
	if err != nil {
		return err
	}

	return nil
}

// VerifyRange verifies all chunks in the results of the blocks in the given range.
// Note, it returns nil if certain block is not executed, in this case warning will be logged
func VerifyRange(
	from, to uint64,
	chainID flow.ChainID,
	protocolDataDir string, chunkDataPackDir string,
	nWorker uint,
	stopOnMismatch bool,
) (err error) {
	closer, storages, chunkDataPacks, state, verifier, err := initStorages(chainID, protocolDataDir, chunkDataPackDir)
	if err != nil {
		return fmt.Errorf("could not init storages: %w", err)
	}
	defer func() {
		closerErr := closer()
		if closerErr != nil {
			err = errors.Join(err, closerErr)
		}
	}()

	log.Info().Msgf("verifying blocks from %d to %d", from, to)

	root := state.Params().SealedRoot().Height

	if from <= root {
		return fmt.Errorf("cannot verify blocks before the root block, from: %d, root: %d", from, root)
	}

	err = verifyConcurrently(from, to, nWorker, stopOnMismatch, storages.Headers, chunkDataPacks, storages.Results, state, verifier, verifyHeight)
	if err != nil {
		return err
	}

	return nil
}

func verifyConcurrently(
	from, to uint64,
	nWorker uint,
	stopOnMismatch bool,
	headers storage.Headers,
	chunkDataPacks storage.ChunkDataPacks,
	results storage.ExecutionResults,
	state protocol.State,
	verifier module.ChunkVerifier,
	verifyHeight func(uint64, storage.Headers, storage.ChunkDataPacks, storage.ExecutionResults, protocol.State, module.ChunkVerifier, bool) error,
) error {
	tasks := make(chan uint64, int(nWorker))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure cancel is called to release resources

	var lowestErr error
	var lowestErrHeight uint64 = ^uint64(0) // Initialize to max value of uint64
	var mu sync.Mutex                       // To protect access to lowestErr and lowestErrHeight

	lg := util.LogProgress(
		log.Logger,
		util.DefaultLogProgressConfig(
			fmt.Sprintf("verifying heights progress for [%v:%v]", from, to),
			int(to+1-from),
		),
	)

	// Worker function
	worker := func() {
		for {
			select {
			case <-ctx.Done():
				return // Stop processing tasks if context is canceled
			case height, ok := <-tasks:
				if !ok {
					return // Exit if the tasks channel is closed
				}
				log.Info().Uint64("height", height).Msg("verifying height")
				err := verifyHeight(height, headers, chunkDataPacks, results, state, verifier, stopOnMismatch)
				if err != nil {
					log.Error().Uint64("height", height).Err(err).Msg("error encountered while verifying height")

					// when encountered an error, the error might not be from the lowest height that had
					// error, so we need to first cancel the context to stop worker from processing further tasks
					// and wait until all workers are done, which will ensure all the heights before this height
					// that had error are processed. Then we can safely update the lowestErr and lowestErrHeight
					mu.Lock()
					if height < lowestErrHeight {
						lowestErr = err
						lowestErrHeight = height
						cancel() // Cancel context to stop further task dispatch
					}
					mu.Unlock()
				} else {
					log.Info().Uint64("height", height).Msg("verified height successfully")
				}

				lg(1) // log progress
			}
		}
	}

	// Start nWorker workers
	var wg sync.WaitGroup
	for i := 0; i < int(nWorker); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker()
		}()
	}

	// Send tasks to workers
	go func() {
		defer close(tasks) // Close tasks channel once all tasks are pushed
		for height := from; height <= to; height++ {
			select {
			case <-ctx.Done():
				return // Stop pushing tasks if context is canceled
			case tasks <- height:
			}
		}
	}()

	// Wait for all workers to complete
	wg.Wait()

	// Check if there was an error
	if lowestErr != nil {
		log.Error().Uint64("height", lowestErrHeight).Err(lowestErr).Msg("error encountered while verifying height")
		return fmt.Errorf("could not verify height %d: %w", lowestErrHeight, lowestErr)
	}

	return nil
}

func initStorages(chainID flow.ChainID, dataDir string, chunkDataPackDir string) (
	func() error,
	*storage.All,
	storage.ChunkDataPacks,
	protocol.State,
	module.ChunkVerifier,
	error,
) {
	db := common.InitStorage(dataDir)

	storages := common.InitStorages(db)
	state, err := common.InitProtocolState(db, storages)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("could not init protocol state: %w", err)
	}

	// require the chunk data pack data must exist before returning the storage module
	chunkDataPackDB, err := storagepebble.MustOpenDefaultPebbleDB(chunkDataPackDir)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("could not open chunk data pack DB: %w", err)
	}
	chunkDataPacks := store.NewChunkDataPacks(metrics.NewNoopCollector(),
		pebbleimpl.ToDB(chunkDataPackDB), storages.Collections, 1000)

	verifier := makeVerifier(log.Logger, chainID, storages.Headers)
	closer := func() error {
		var dbErr, chunkDataPackDBErr error

		if err := db.Close(); err != nil {
			dbErr = fmt.Errorf("failed to close protocol db: %w", err)
		}

		if err := chunkDataPackDB.Close(); err != nil {
			chunkDataPackDBErr = fmt.Errorf("failed to close chunk data pack db: %w", err)
		}
		return errors.Join(dbErr, chunkDataPackDBErr)
	}
	return closer, storages, chunkDataPacks, state, verifier, nil
}

// verifyHeight verifies all chunks in the results of the block at the given height.
// Note: it returns nil if the block is not executed.
func verifyHeight(
	height uint64,
	headers storage.Headers,
	chunkDataPacks storage.ChunkDataPacks,
	results storage.ExecutionResults,
	state protocol.State,
	verifier module.ChunkVerifier,
	stopOnMismatch bool,
) error {
	header, err := headers.ByHeight(height)
	if err != nil {
		return fmt.Errorf("could not get block header by height %d: %w", height, err)
	}

	blockID := header.ID()

	result, err := results.ByBlockID(blockID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			log.Warn().Uint64("height", height).Hex("block_id", blockID[:]).Msg("execution result not found")
			return nil
		}

		return fmt.Errorf("could not get execution result by block ID %s: %w", blockID, err)
	}
	snapshot := state.AtBlockID(blockID)

	for i, chunk := range result.Chunks {
		chunkDataPack, err := chunkDataPacks.ByChunkID(chunk.ID())
		if err != nil {
			return fmt.Errorf("could not get chunk data pack by chunk ID %s: %w", chunk.ID(), err)
		}

		vcd, err := convert.FromChunkDataPack(chunk, chunkDataPack, header, snapshot, result)
		if err != nil {
			return err
		}

		_, err = verifier.Verify(vcd)
		if err != nil {
			if stopOnMismatch {
				return fmt.Errorf("could not verify chunk (index: %v) at block %v (%v): %w", i, height, blockID, err)
			}

			log.Error().Err(err).Msgf("could not verify chunk (index: %v) at block %v (%v)", i, height, blockID)
		}
	}
	return nil
}

func makeVerifier(
	logger zerolog.Logger,
	chainID flow.ChainID,
	headers storage.Headers,
) module.ChunkVerifier {

	vm := fvm.NewVirtualMachine()
	fvmOptions := initialize.InitFvmOptions(chainID, headers)
	fvmOptions = append(
		[]fvm.Option{fvm.WithLogger(logger)},
		fvmOptions...,
	)

	// TODO(JanezP): cleanup creation of fvm context github.com/onflow/flow-go/issues/5249
	fvmOptions = append(fvmOptions, computation.DefaultFVMOptions(chainID, false, false)...)
	vmCtx := fvm.NewContext(fvmOptions...)

	chunkVerifier := chunks.NewChunkVerifier(vm, vmCtx, logger)
	return chunkVerifier
}
