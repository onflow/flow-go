package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	findBlockByCommits "github.com/onflow/flow-go/cmd/util/cmd/read-badger/cmd/find-block-by-commits"
	"github.com/onflow/flow-go/model/flow"
)

func init() {
	rootCmd.AddCommand(commitsCmd)

	commitsCmd.Flags().StringVarP(&flagBlockID, "block-id", "b", "", "the block id of which to query the state commitment")
	_ = commitsCmd.MarkFlagRequired("block-id")

	rootCmd.AddCommand(findBlockByCommits.Init(InitStorages))
}

var commitsCmd = &cobra.Command{
	Use:   "commits",
	Short: "get commit by block ID",
	Run: func(cmd *cobra.Command, args []string) {
		storages, db := InitStorages()
		defer db.Close()

		log.Info().Msgf("got flag block id: %s", flagBlockID)
		blockID, err := flow.HexStringToIdentifier(flagBlockID)
		if err != nil {
			log.Error().Err(err).Msg("malformed block id")
			return
		}

		log.Info().Msgf("getting commit by block id: %v", blockID)
		commit, err := storages.Commits.ByBlockID(blockID)
		if err != nil {
			log.Error().Err(err).Msgf("could not get commit for block id: %v", blockID)
			return
		}

		log.Info().Msgf("commit: %x", commit)
	},
}
