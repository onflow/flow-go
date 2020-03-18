package cmd

import (
	"fmt"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var privKey string

// stateCmd represents the state command
var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "Generate genesis execution state",
	Run: func(cmd *cobra.Command, args []string) {
		// TODO validation of privKey

		stateCommitment, err := run.GenerateExecutionState(privKey)
		if err != nil {
			log.Fatal().Err(err).Msg("error generating execution state")
		}

		writeYaml("state.yml", fmt.Sprintf("%#x", stateCommitment))
	},
}

func init() {
	rootCmd.AddCommand(stateCmd)

	stateCmd.Flags().StringVarP(&privKey, "account-0-priv", "a", "", "Hex encoded private key of account 0 [required]")
	stateCmd.MarkFlagRequired("account-0-priv")
}
