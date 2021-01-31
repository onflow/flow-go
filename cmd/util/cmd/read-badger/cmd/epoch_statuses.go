package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
)

func init() {
	rootCmd.AddCommand(epochStatusesCmd)

	epochStatusesCmd.Flags().StringVarP(&flagBlockID, "block-id", "b", "", "the block id of which to query the epoch status")
	_ = epochStatusesCmd.MarkFlagRequired("block-id")
}

var epochStatusesCmd = &cobra.Command{
	Use:   "epoch-statuses",
	Short: "get epoch statuses by block ID",
	Run: func(cmd *cobra.Command, args []string) {
		storages, db := InitStorages()
		defer db.Close()

		log.Info().Msgf("got flag block id: %s", flagBlockID)
		blockID, err := flow.HexStringToIdentifier(flagBlockID)
		if err != nil {
			log.Fatal().Err(err).Msg("malformed block id")
		}

		log.Info().Msgf("getting epoch status by block id: %v", blockID)
		epochStatus, err := storages.Statuses.ByBlockID(blockID)
		if err != nil {
			log.Fatal().Err(err).Msgf("could not get epoch status for block id: %v", blockID)
		}

		common.PrettyPrint(epochStatus)
	},
}
