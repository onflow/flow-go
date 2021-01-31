package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
)

var flagEpochCommitID string

func init() {
	rootCmd.AddCommand(epochCommitCmd)

	epochCommitCmd.Flags().StringVarP(&flagEpochCommitID, "d", "i", "", "the id of the epoch commit")
	_ = epochCommitCmd.MarkFlagRequired("id")
}

var epochCommitCmd = &cobra.Command{
	Use:   "epoch-commit",
	Short: "get epoch commit by ID",
	Run: func(cmd *cobra.Command, args []string) {
		storages, db := InitStorages()
		defer db.Close()

		log.Info().Msgf("got flag commit id: %s", flagEpochCommitID)
		commitID, err := flow.HexStringToIdentifier(flagEpochCommitID)
		if err != nil {
			log.Fatal().Err(err).Msg("malformed epoch commit id")
		}

		log.Info().Msgf("getting epoch commit by id: %v", commitID)
		epochCommit, err := storages.EpochCommits.ByID(commitID)
		if err != nil {
			log.Fatal().Err(err).Msgf("could not get epoch commit with id: %v", commitID)
		}

		log.Info().Msgf("epoch commit id: %v", epochCommit.ID())
		common.PrettyPrint(epochCommit)
	},
}
