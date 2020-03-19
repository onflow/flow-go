package cmd

import (
	"encoding/hex"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var privKey string

type StateCommitment struct {
	flow.StateCommitment
}

func (sc StateCommitment) MarshalYAML() (interface{}, error) {
	return hex.EncodeToString(sc.StateCommitment), nil
}

func (sc *StateCommitment) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	err := unmarshal(&s)
	if err != nil {
		return err
	}
	sc.StateCommitment, err = hex.DecodeString(s)
	return err
}

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

		writeYaml("state.yml", StateCommitment{stateCommitment})
	},
}

func init() {
	rootCmd.AddCommand(stateCmd)

	stateCmd.Flags().StringVarP(&privKey, "account-0-priv", "a", "", "Hex encoded private key of account 0 [required]")
	stateCmd.MarkFlagRequired("account-0-priv")
}
