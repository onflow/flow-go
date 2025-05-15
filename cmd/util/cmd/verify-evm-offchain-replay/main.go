package verify

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/model/flow"
)

var (
	flagDatadir          string
	flagExecutionDataDir string
	flagEVMStateGobDir   string
	flagChain            string
	flagFromTo           string
	flagSaveEveryNBlocks uint64
)

// usage example
//
//		./util verify-evm-offchain-replay --chain flow-testnet --from_to 211176670-211177000
//	     --datadir /var/flow/data/protocol --execution_data_dir /var/flow/data/execution_data
var Cmd = &cobra.Command{
	Use:   "verify-evm-offchain-replay",
	Short: "verify evm offchain replay with execution data",
	Run:   run,
}

func init() {
	Cmd.Flags().StringVar(&flagChain, "chain", "", "Chain name")
	_ = Cmd.MarkFlagRequired("chain")

	Cmd.Flags().StringVar(&flagDatadir, "datadir", "/var/flow/data/protocol",
		"directory that stores the protocol state")

	Cmd.Flags().StringVar(&flagExecutionDataDir, "execution_data_dir", "/var/flow/data/execution_data",
		"directory that stores the execution state")

	Cmd.Flags().StringVar(&flagFromTo, "from_to", "",
		"the flow height range to verify blocks, i.e, 1-1000, 1000-2000, 2000-3000, etc.")

	Cmd.Flags().StringVar(&flagEVMStateGobDir, "evm_state_gob_dir", "/var/flow/data/evm_state_gob",
		"directory that stores the evm state gob files as checkpoint")

	Cmd.Flags().Uint64Var(&flagSaveEveryNBlocks, "save_every", uint64(1_000_000),
		"save the evm state gob files every N blocks")
}

func run(*cobra.Command, []string) {
	chainID := flow.ChainID(flagChain)

	from, to, err := parseFromTo(flagFromTo)
	if err != nil {
		log.Fatal().Err(err).Msg("could not parse from_to")
	}

	err = Verify(log.Logger, from, to, chainID, flagDatadir, flagExecutionDataDir, flagEVMStateGobDir, flagSaveEveryNBlocks)
	if err != nil {
		log.Fatal().Err(err).Msg("could not verify height")
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
