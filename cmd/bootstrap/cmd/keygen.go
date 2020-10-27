package cmd

import (
	"github.com/spf13/cobra"

	model "github.com/onflow/flow-go/model/bootstrap"
)

// keygenCmd represents the key gen command
var keygenCmd = &cobra.Command{
	Use:   "keygen",
	Short: "Create all keys on ansible machine for the new network",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		log.Info().Msg("generating internal private networking and staking keys")
		_ = genNetworkAndStakingKeys([]model.NodeInfo{})
		log.Info().Msg("")
	},
}

func init() {
	rootCmd.AddCommand(keygenCmd)

	// required parameters
	keygenCmd.Flags().
		StringVar(&flagConfig, "config", "", "path to a JSON file containing multiple node configurations (Role, Address, Stake)")
	_ = keygenCmd.MarkFlagRequired("config")

	// optional parameters
	keygenCmd.Flags().
		BoolVar(&flagFastKG, "fast-kg", false, "use fast (centralized) random beacon key generation instead of DKG")
}
