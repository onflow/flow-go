package inspect

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/fvm/inspection"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

var (
	flagDatadir          string
	flagChunkDataPackDir string
	flagChain            string
	flagFromTo           string
	flagLastK            uint64
)

// Cmd is the command for inspecting token movements in executed blocks
// by reading chunk data packs and running the token changes inspector.
//
// # inspect the last 100 sealed blocks
// ./util inspect-token-movements --chain flow-mainnet --datadir /var/flow/data/protocol --chunk_data_pack_dir /var/flow/data/chunk_data_packs --lastk 100
// # inspect the blocks from height 2000 to 3000
// ./util inspect-token-movements --chain flow-mainnet --datadir /var/flow/data/protocol --chunk_data_pack_dir /var/flow/data/chunk_data_packs --from_to 2000_3000
var Cmd = &cobra.Command{
	Use:   "inspect-token-movements",
	Short: "inspect token movements by analyzing chunk data packs for unaccounted token mints/burns",
	Run:   run,
}

func init() {
	Cmd.Flags().StringVar(&flagChain, "chain", "", "Chain name")
	_ = Cmd.MarkFlagRequired("chain")

	common.InitDataDirFlag(Cmd, &flagDatadir)

	Cmd.Flags().StringVar(&flagChunkDataPackDir, "chunk_data_pack_dir", "/var/flow/data/chunk_data_packs",
		"directory that stores the chunk data packs")
	_ = Cmd.MarkFlagRequired("chunk_data_pack_dir")

	Cmd.Flags().Uint64Var(&flagLastK, "lastk", 1,
		"last k sealed blocks to inspect")

	Cmd.Flags().StringVar(&flagFromTo, "from_to", "",
		"the height range to inspect blocks (inclusive), i.e, 1_1000, 1000_2000, 2000_3000, etc.")
}

func run(*cobra.Command, []string) {
	lockManager := storage.MakeSingletonLockManager()
	chainID := flow.ChainID(flagChain)
	chain := chainID.Chain()

	lg := log.With().
		Str("chain", string(chainID)).
		Str("datadir", flagDatadir).
		Str("chunk_data_pack_dir", flagChunkDataPackDir).
		Uint64("lastk", flagLastK).
		Str("from_to", flagFromTo).
		Logger()

	lg.Info().Msg("initializing token movements inspector")

	closer, storages, chunkDataPacks, state, err := initStorages(lockManager, flagDatadir, flagChunkDataPackDir)
	if err != nil {
		lg.Fatal().Err(err).Msg("could not init storages")
	}
	defer func() {
		if closeErr := closer(); closeErr != nil {
			lg.Warn().Err(closeErr).Msg("error closing storages")
		}
	}()

	// Create the token changes inspector with default search tokens for this chain
	inspector := inspection.NewTokenChangesInspector(inspection.DefaultTokenDiffSearchTokens(chain, true), chainID)

	var from, to uint64

	lastSealed, err := state.Sealed().Head()
	if err != nil {
		lg.Fatal().Err(err).Msg("could not get last sealed height")
	}

	if flagFromTo != "" {
		from, to, err = parseFromTo(flagFromTo)
		if err != nil {
			lg.Fatal().Err(err).Msg("could not parse from_to")
		}

		if to > lastSealed.Height {
			lg.Fatal().Msgf("'to' height (%d) exceeds last sealed block height (%d)", to, lastSealed.Height)
		}
	} else {
		root := state.Params().SealedRoot().Height

		// preventing overflow
		if flagLastK > lastSealed.Height+1 {
			lg.Fatal().Msgf("k is greater than the number of sealed blocks, k: %d, last sealed height: %d", flagLastK, lastSealed.Height)
		}

		from = lastSealed.Height - flagLastK + 1

		// root block is not verifiable, because it's sealed already.
		// the first verifiable is the next block of the root block
		firstVerifiable := root + 1

		if from < firstVerifiable {
			from = firstVerifiable
		}
		to = lastSealed.Height
	}

	root := state.Params().SealedRoot().Height
	if from <= root {
		lg.Fatal().Msgf("cannot inspect blocks before the root block, from: %d, root: %d", from, root)
	}

	lg.Info().Msgf("inspecting token movements for blocks from %d to %d", from, to)

	for height := from; height <= to; height++ {
		err := inspectHeight(
			lg,
			chainID,
			height,
			storages.Headers,
			chunkDataPacks,
			storages.Results,
			state,
			inspector,
		)
		if err != nil {
			lg.Error().Err(err).Uint64("height", height).Msg("error inspecting height")
		}
	}

	lg.Info().Msgf("finished inspecting token movements for blocks from %d to %d", from, to)
}

func inspectHeight(
	lg zerolog.Logger,
	chainID flow.ChainID,
	height uint64,
	headers storage.Headers,
	chunkDataPacks storage.ChunkDataPacks,
	results storage.ExecutionResults,
	protocolState protocol.State,
	inspector *inspection.TokenChanges,
) error {
	header, err := headers.ByHeight(height)
	if err != nil {
		return fmt.Errorf("could not get block header by height %d: %w", height, err)
	}

	blockID := header.ID()

	result, err := results.ByBlockID(blockID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			lg.Warn().Uint64("height", height).Hex("block_id", blockID[:]).Msg("execution result not found")
			return nil
		}
		return fmt.Errorf("could not get execution result by block ID %s: %w", blockID, err)
	}

	heightLg := lg.With().
		Uint64("height", height).
		Hex("block_id", blockID[:]).
		Logger()

	heightLg.Info().Int("num_chunks", len(result.Chunks)).Msg("inspecting block")

	for _, chunk := range result.Chunks {
		chunkDataPack, err := chunkDataPacks.ByChunkID(chunk.ID())
		if err != nil {
			return fmt.Errorf("could not get chunk data pack by chunk ID %s: %w", chunk.ID(), err)
		}

		chunkLg := heightLg.With().
			Uint64("chunk_index", chunk.Index).
			Logger()

		err = inspectChunkFromDataPack(
			chunkLg,
			chainID,
			header,
			chunk,
			chunkDataPack,
			result,
			protocolState,
			headers,
			inspector,
		)
		if err != nil {
			chunkLg.Error().Err(err).Msg("error inspecting chunk")
		}
	}

	return nil
}

func parseFromTo(fromTo string) (from, to uint64, err error) {
	parts := strings.Split(fromTo, "_")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid format: expected 'from_to', got '%s'", fromTo)
	}

	from, err = strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid 'from' value: %w", err)
	}

	to, err = strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid 'to' value: %w", err)
	}

	if from > to {
		return 0, 0, fmt.Errorf("'from' value (%d) must be less than or equal to 'to' value (%d)", from, to)
	}

	return from, to, nil
}
