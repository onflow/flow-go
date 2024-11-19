package verifier

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/environment"
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
func VerifyLastKHeight(k uint64, chainID flow.ChainID, protocolDataDir string, chunkDataPackDir string) error {
	db, storages, chunkDataPacks, state, verifier, err := initStorages(chainID, protocolDataDir, chunkDataPackDir)
	if err != nil {
		return fmt.Errorf("could not init storages: %w", err)
	}
	defer db.Close()

	lastSealed, err := state.Sealed().Head()
	if err != nil {
		return fmt.Errorf("could not get last sealed height: %w", err)
	}

	root := state.Params().SealedRoot().Height
	from := lastSealed.Height - k + 1

	// root block is not verifiable, because it's sealed already.
	// the first verifiable is the next block of the root block
	firstVerifiable := root + 1

	if from < firstVerifiable {
		from = firstVerifiable
	}
	to := lastSealed.Height

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
func VerifyRange(
	from, to uint64,
	chainID flow.ChainID,
	protocolDataDir string, chunkDataPackDir string,
) error {
	db, storages, chunkDataPacks, state, verifier, err := initStorages(chainID, protocolDataDir, chunkDataPackDir)
	if err != nil {
		return fmt.Errorf("could not init storages: %w", err)
	}
	defer db.Close()

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
	*badger.DB,
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

	chunkDataPackDB, err := storagepebble.OpenDefaultPebbleDB(chunkDataPackDir)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("could not open chunk data pack DB: %w", err)
	}
	chunkDataPacks := storagepebble.NewChunkDataPacks(metrics.NewNoopCollector(),
		chunkDataPackDB, storages.Collections, 1000)

	verifier := makeVerifier(log.Logger, chainID, storages.Headers)
	return db, storages, chunkDataPacks, state, verifier, nil
}

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

	if err != nil {
		return fmt.Errorf("could not get block ID by height %d: %w", height, err)
	}

	result, err := results.ByBlockID(blockID)
	if err != nil {
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
	fvmOptions := initFvmOptions(chainID, headers)
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

func initFvmOptions(chainID flow.ChainID, headers storage.Headers) []fvm.Option {
	blockFinder := environment.NewBlockFinder(headers)
	vmOpts := []fvm.Option{
		fvm.WithChain(chainID.Chain()),
		fvm.WithBlocks(blockFinder),
		fvm.WithAccountStorageLimit(true),
	}
	switch chainID {
	case flow.Testnet,
		flow.Sandboxnet,
		flow.Previewnet,
		flow.Mainnet:
		vmOpts = append(vmOpts,
			fvm.WithTransactionFeesEnabled(true),
		)
	}
	switch chainID {
	case flow.Testnet,
		flow.Sandboxnet,
		flow.Previewnet,
		flow.Localnet,
		flow.Benchnet:
		vmOpts = append(vmOpts,
			fvm.WithContractDeploymentRestricted(false),
		)
	}
	return vmOpts
}
