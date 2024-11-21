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
)

// usage example
//
//		./util verify-evm-offchain-replay --chain flow-testnet --from-to 211176671-211177000
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
}

func run(*cobra.Command, []string) {
	_ = flow.ChainID(flagChain).Chain()

	from, to, err := parseFromTo(flagFromTo)
	if err != nil {
		log.Fatal().Err(err).Msg("could not parse from_to")
	}

	log.Info().Msgf("verifying range from %d to %d", from, to)
	err = Verify(from, to, flow.Testnet, flagDatadir, flagExecutionDataDir, flagEVMStateGobDir)
	if err != nil {
		log.Fatal().Err(err).Msg("could not verify last k height")
	}
	log.Info().Msgf("successfully verified range from %d to %d", from, to)

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
