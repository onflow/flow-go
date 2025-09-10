package cmd

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store"
)

func init() {
	rootCmd.AddCommand(commitsCmd)

	commitsCmd.Flags().StringVarP(&flagBlockID, "block-id", "b", "", "the block id of which to query the state commitment")
	_ = commitsCmd.MarkFlagRequired("block-id")
}

var commitsCmd = &cobra.Command{
	Use:   "commits",
	Short: "get commit by block ID",
	Run: func(cmd *cobra.Command, args []string) {
		err := WithStorage(func(db storage.DB) error {

			commits := store.NewCommits(metrics.NewNoopCollector(), db)

			log.Info().Msgf("got flag block id: %s", flagBlockID)
			blockID, err := flow.HexStringToIdentifier(flagBlockID)
			if err != nil {
				return fmt.Errorf("malformed block id: %w", err)
			}

			log.Info().Msgf("getting commit by block id: %v", blockID)
			commit, err := commits.ByBlockID(blockID)
			if err != nil {
				return fmt.Errorf("could not get commit for block id: %v: %w", blockID, err)
			}

			log.Info().Msgf("commit: %v", commit)
			return nil
		})

		if err != nil {
			log.Error().Err(err).Msg("could not get events")
		}
	},
}
