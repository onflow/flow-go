package cmd

import (
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/model/flow"
)

// wrapCmd represents a command to (Flow Team Use), wrap response keys for consensus node
var wrapCmd = &cobra.Command{
	Use:   "wrap",
	Short: "(Flow Team Use), wrap response keys for consensus node",
	Long:  `(Flow Team Use), wrap response keys for consensus node`,
	Run:   wrap,
}

func init() {
	rootCmd.AddCommand(wrapCmd)
	addWrapCmdFlags()
}

func addWrapCmdFlags() {
	wrapCmd.Flags().StringVarP(&flagWrapID, "wrap-id", "i", "", "(Flow Team Use), wrap response keys for consensus node")
	wrapCmd.Flags().StringVarP(&flagNodeRole, "role", "r", "", `node role (can be "collection", "consensus", "execution", "verification", "observer" or "access")`)

	_ = wrapCmd.MarkFlagRequired("wrap-id")
	_ = wrapCmd.MarkFlagRequired("role")
}

// wrap wraps response keys for consensus node
func wrap(cmd *cobra.Command, args []string) {
	log.Info().Msg("running wrap")

	role, err := flow.ParseRole(flagNodeRole)
	if err != nil {
		log.Fatal().Err(err).Msg("could not parse Flow role")
	}

	if role != flow.RoleConsensus {
		log.Info().Str("role", role.String()).Msg("wrap should be only performed if node role is consensus")
		return
	}

	log.Info().Str("wrap_id", flagWrapID).Msgf("wrapping response for node")
	err = wrapFile(flagBootDir, flagWrapID)
	if err != nil {
		log.Fatal().Err(err).Msgf("failed to wrap response")
	}

	log.Info().Msgf("wrapping completed")
}
