package cmd

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store"
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
	RunE: func(cmd *cobra.Command, args []string) error {
		return common.WithStorage(flagDatadir, func(db storage.DB) error {
			epochCommits := store.NewEpochCommits(metrics.NewNoopCollector(), db)

			log.Info().Msgf("got flag commit id: %s", flagEpochCommitID)
			commitID, err := flow.HexStringToIdentifier(flagEpochCommitID)
			if err != nil {
				return fmt.Errorf("malformed epoch commit id: %w", err)
			}

			log.Info().Msgf("getting epoch commit by id: %v", commitID)
			epochCommit, err := epochCommits.ByID(commitID)
			if err != nil {
				return fmt.Errorf("could not get epoch commit with id: %v: %w", commitID, err)
			}

			log.Info().Msgf("epoch commit id: %v", epochCommit.ID())
			common.PrettyPrint(epochCommit)
			return nil
		})
	},
}
