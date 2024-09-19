package debug_tx

import (
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/debug"
)

// use the following command to forward port 9000 from the EN to localhost:9001
// `gcloud compute ssh '--ssh-flag=-A' --no-user-output-enabled --tunnel-through-iap migrationmainnet1-execution-001 --project flow-multi-region -- -NL 9001:localhost:9000`

var (
	flagExecutionAddress string
	flagChain            string
	flagScript           string
)

var Cmd = &cobra.Command{
	Use:   "debug-script",
	Short: "debug a script",
	Run:   run,
}

func init() {

	Cmd.Flags().StringVar(
		&flagChain,
		"chain",
		"",
		"Chain name",
	)
	_ = Cmd.MarkFlagRequired("chain")

	Cmd.Flags().StringVar(&flagExecutionAddress, "execution-address", "", "address of the execution node")
	_ = Cmd.MarkFlagRequired("execution-address")

	Cmd.Flags().StringVar(&flagScript, "script", "", "path to script")
	_ = Cmd.MarkFlagRequired("script")
}

func run(*cobra.Command, []string) {

	chainID := flow.ChainID(flagChain)
	chain := chainID.Chain()

	code, err := os.ReadFile(flagScript)
	if err != nil {
		log.Fatal().Err(err).Msgf("failed to read script from file %s", flagScript)
	}

	debugger := debug.NewRemoteDebugger(
		flagExecutionAddress,
		chain,
		log.Logger,
	)

	// TODO: add support for arguments
	var arguments [][]byte

	result, scriptErr, processErr := debugger.RunScript(code, arguments)

	if scriptErr != nil {
		log.Fatal().Err(scriptErr).Msg("transaction error")
	}
	if processErr != nil {
		log.Fatal().Err(processErr).Msg("process error")
	}
	log.Info().Msgf("result: %s", result)
}
