package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/model/flow"
)

// pushCmd represents a command to upload public keys to the transit server
var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Upload public keys to the transit server",
	Long:  `Upload public keys to the transit server`,
	Run:   push,
}

func init() {
	rootCmd.AddCommand(pushCmd)
	addPushCmdFlags()
}

func addPushCmdFlags() {
	pushCmd.Flags().StringVarP(&flagToken, "token", "t", "", "token provided by the Flow team to access the Transit server")
	pushCmd.Flags().StringVarP(&flagNodeRole, "role", "r", "", `node role (can be "collection", "consensus", "execution", "verification" or "access")`)
	pushCmd.Flags().StringVarP(&flagNodeID, "nodeID", "n", "", "node id")
	pushCmd.Flags().StringVarP(&flagOutputDir, "outputDir", "o", "", "ouput directory")
	_ = pushCmd.MarkFlagRequired("token")
	_ = pushCmd.MarkFlagRequired("nodeID")
}

// push uploads public keys to the transit server
func push(_ *cobra.Command, _ []string) {
	if flagNodeRole != flow.RoleConsensus.String() {
		log.Info().Str("role", flagNodeRole).Msgf("only consensus nodes are required to push transit keys, exiting.")
		os.Exit(0)
	}

	log.Info().Msg("running push")

	// ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	// defer cancel()
	//
	nodeID := flagNodeID

	err := generateKeys(flagOutputDir, nodeID)
	if err != nil {
		log.Fatal().Err(err).Msg(err.Error())
	}

	log.Info().Msg("successfully pushed transit public key to the transit servers")
}
