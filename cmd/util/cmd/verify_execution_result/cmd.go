package verify

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/engine/verification/verifier"
	"github.com/onflow/flow-go/model/flow"
)

var (
	flagLastK            uint64
	flagDatadir          string
	flagChunkDataPackDir string
	flagChain            string
	flagFromTo           string
	flagWorkerCount      uint // number of workers to verify the blocks concurrently
	flagStopOnMismatch   bool
)

// # verify the last 100 sealed blocks
// ./util verify_execution_result --chain flow-testnet --datadir /var/flow/data/protocol --chunk_data_pack_dir /var/flow/data/chunk_data_pack --lastk 100
// # verify the blocks from height 2000 to 3000
// ./util verify_execution_result --chain flow-testnet --datadir /var/flow/data/protocol --chunk_data_pack_dir /var/flow/data/chunk_data_pack --from_to 2000_3000
var Cmd = &cobra.Command{
	Use:   "verify-execution-result",
	Short: "verify block execution by verifying all chunks in the result",
	Run:   run,
}

func init() {
	Cmd.Flags().StringVar(&flagChain, "chain", "", "Chain name")
	_ = Cmd.MarkFlagRequired("chain")

	Cmd.Flags().StringVar(&flagDatadir, "datadir", "/var/flow/data/protocol",
		"directory that stores the protocol state")
	_ = Cmd.MarkFlagRequired("datadir")

	Cmd.Flags().StringVar(&flagChunkDataPackDir, "chunk_data_pack_dir", "/var/flow/data/chunk_data_pack",
		"directory that stores the protocol state")
	_ = Cmd.MarkFlagRequired("chunk_data_pack_dir")

	Cmd.Flags().Uint64Var(&flagLastK, "lastk", 1,
		"last k sealed blocks to verify")

	Cmd.Flags().StringVar(&flagFromTo, "from_to", "",
		"the height range to verify blocks (inclusive), i.e, 1_1000, 1000_2000, 2000_3000, etc.")

	Cmd.Flags().UintVar(&flagWorkerCount, "worker_count", 1,
		"number of workers to use for verification, default is 1")

	Cmd.Flags().BoolVar(&flagStopOnMismatch, "stop_on_mismatch", false, "stop verification on first mismatch")
}

func run(*cobra.Command, []string) {
	chainID := flow.ChainID(flagChain)
	_ = chainID.Chain()

	if flagWorkerCount < 1 {
		log.Fatal().Msgf("worker count must be at least 1, but got %v", flagWorkerCount)
	}

	lg := log.With().
		Str("chain", string(chainID)).
		Str("datadir", flagDatadir).
		Str("chunk_data_pack_dir", flagChunkDataPackDir).
		Uint64("lastk", flagLastK).
		Str("from_to", flagFromTo).
		Uint("worker_count", flagWorkerCount).
		Bool("stop_on_mismatch", flagStopOnMismatch).
		Logger()

	if flagFromTo != "" {
		from, to, err := parseFromTo(flagFromTo)
		if err != nil {
			lg.Fatal().Err(err).Msg("could not parse from_to")
		}

		lg.Info().Msgf("verifying range from %d to %d", from, to)
		err = verifier.VerifyRange(from, to, chainID, flagDatadir, flagChunkDataPackDir, flagWorkerCount, flagStopOnMismatch)
		if err != nil {
			lg.Fatal().Err(err).Msgf("could not verify range from %d to %d", from, to)
		}
		lg.Info().Msgf("successfully verified range from %d to %d", from, to)

	} else {
		lg.Info().Msgf("verifying last %d sealed blocks", flagLastK)
		err := verifier.VerifyLastKHeight(flagLastK, chainID, flagDatadir, flagChunkDataPackDir, flagWorkerCount, flagStopOnMismatch)
		if err != nil {
			lg.Fatal().Err(err).Msg("could not verify last k height")
		}

		lg.Info().Msgf("successfully verified last %d sealed blocks", flagLastK)
	}
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
