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
)

// # verify the last 100 sealed blocks
// ./util verify_execution_result --chain flow-testnet --datadir /var/flow/data/protocol --chunk_data_pack_dir /var/flow/data/chunk_data_pack --lastk 100
// # verify the blocks from height 2000 to 3000
// ./util verify_execution_result --chain flow-testnet --datadir /var/flow/data/protocol --chunk_data_pack_dir /var/flow/data/chunk_data_pack --from_to 2000-3000
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
		"the height range to verify blocks (inclusive), i.e, 1-1000, 1000-2000, 2000-3000, etc.")
}

func run(*cobra.Command, []string) {
	chainID := flow.ChainID(flagChain)

	if flagFromTo != "" {
		from, to, err := parseFromTo(flagFromTo)
		if err != nil {
			log.Fatal().Err(err).Msg("could not parse from_to")
		}

		log.Info().Msgf("verifying range from %d to %d", from, to)
		err = verifier.VerifyRange(from, to, chainID, flagDatadir, flagChunkDataPackDir)
		if err != nil {
			log.Fatal().Err(err).Msg("could not verify range from %d to %d")
		}
		log.Info().Msgf("successfully verified range from %d to %d", from, to)

	} else {
		log.Info().Msgf("verifying last %d sealed blocks", flagLastK)
		err := verifier.VerifyLastKHeight(flagLastK, chainID, flagDatadir, flagChunkDataPackDir)
		if err != nil {
			log.Fatal().Err(err).Msg("could not verify last k height")
		}

		log.Info().Msgf("successfully verified last %d sealed blocks", flagLastK)
	}
}

func parseFromTo(fromTo string) (from, to uint64, err error) {
	parts := strings.Split(fromTo, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid format: expected 'from-to', got '%s'", fromTo)
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
