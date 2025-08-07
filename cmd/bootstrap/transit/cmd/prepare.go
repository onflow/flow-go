package cmd

import (
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/model/flow"
)

// prepareCmd represents a command to generate transit keys for push command
var prepareCmd = &cobra.Command{
	Use:   "prepare",
	Short: "Generate transit keys for push command (only needed for consensus node)",
	Long:  `Generate transit keys for push command`,
	Run:   prepare,
}

func init() {
	rootCmd.AddCommand(prepareCmd)
	addPrepareCmdFlags()
}

func addPrepareCmdFlags() {
	prepareCmd.Flags().StringVarP(&flagNodeRole, "role", "r", "", `node role (can be "collection", "consensus", "execution", "verification" or "access")`)
	prepareCmd.Flags().StringVarP(&flagNodeID, "nodeID", "n", "", "node id")
	prepareCmd.Flags().StringVarP(&flagOutputDir, "outputDir", "o", "", "ouput directory")
	_ = prepareCmd.MarkFlagRequired("role")
}

// prepare generates transit keys for push command
func prepare(cmd *cobra.Command, args []string) {
	log.Info().Msg("running prepare")

	role, err := flow.ParseRole(flagNodeRole)
	if err != nil {
		log.Fatal().Err(err).Msg("could not parse Flow role")
	}

	if role != flow.RoleConsensus {
		log.Info().Str("role", role.String()).Msg("no preparation needed for role")
		return
	}

	// Set the output directory from the flag or use the bootstrap directory
	outputDir := flagOutputDir
	if outputDir == "" {
		outputDir = flagBootDir
	}

	// Set the NodeID from the flag or read it from the file
	nodeID := flagNodeID
	if nodeID == "" {
		nodeID, err = readNodeID()
		if err != nil {
			log.Fatal().Err(err).Msg("could not read node ID from file")
		}
	}

	err = generateKeys(outputDir, nodeID)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to prepare")
	}
	log.Info().Str("role", role.String()).Msg("completed preparation")
}
