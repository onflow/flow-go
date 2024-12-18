package verifier

import (
	"errors"
	"fmt"

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
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	storagepebble "github.com/onflow/flow-go/storage/pebble"
)

// VerifyLastKHeight verifies the last k sealed blocks by verifying all chunks in the results.
// It assumes the latest sealed block has been executed, and the chunk data packs have not been
// pruned.
// Note, it returns nil if certain block is not executed, in this case warning will be logged
func VerifyLastKHeight(k uint64, chainID flow.ChainID, protocolDataDir string, chunkDataPackDir string) (err error) {
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

	for height := from; height <= to; height++ {
		log.Info().Uint64("height", height).Msg("verifying height")
		err := verifyHeight(height, storages.Headers, chunkDataPacks, storages.Results, state, verifier)
		if err != nil {
			return fmt.Errorf("could not verify height %d: %w", height, err)
		}
	}

	return nil
}

// VerifyRange verifies all chunks in the results of the blocks in the given range.
// Note, it returns nil if certain block is not executed, in this case warning will be logged
func VerifyRange(
	from, to uint64,
	chainID flow.ChainID,
	protocolDataDir string, chunkDataPackDir string,
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

	for height := from; height <= to; height++ {
		log.Info().Uint64("height", height).Msg("verifying height")
		err := verifyHeight(height, storages.Headers, chunkDataPacks, storages.Results, state, verifier)
		if err != nil {
			return fmt.Errorf("could not verify height %d: %w", height, err)
		}
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
	chunkDataPacks := storagepebble.NewChunkDataPacks(metrics.NewNoopCollector(),
		chunkDataPackDB, storages.Collections, 1000)

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
			return fmt.Errorf("could not verify %d-th chunk: %w", i, err)
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
