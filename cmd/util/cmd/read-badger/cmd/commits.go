package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/model/flow"
)

func init() {
	rootCmd.AddCommand(commitsCmd)

	commitsCmd.Flags().StringVarP(&flagBlockID, "block-id", "b", "", "the block identifier of which to query the state commitment")
	_ = commitsCmd.MarkFlagRequired("block-id")
}

var commitsCmd = &cobra.Command{
	Use:   "commits",
	Short: "get commit by block ID",
	Run: func(cmd *cobra.Command, args []string) {
		storages := InitStorages()

		blockID, err := flow.HexStringToIdentifier(flagBlockID)
		if err != nil {
			log.Fatal().Err(err).Msg("malformed block identifier")
		}

		log.Info().Msgf("getting commit by block id: %v", blockID)
		commit, err := storages.Commits.ByBlockID(blockID)
		if err != nil {
			log.Fatal().Err(err).Msgf("could not get commit")
		}

		log.Info().Msgf("commit: %x", commit)
	},
}
