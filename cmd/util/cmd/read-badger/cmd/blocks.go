package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
)

var flagBlockID string

func init() {
	rootCmd.AddCommand(blocksCmd)

	blocksCmd.Flags().StringVarP(&flagBlockID, "id", "i", "", "the id of the block")
	_ = blocksCmd.MarkFlagRequired("id")
}

var blocksCmd = &cobra.Command{
	Use:   "blocks",
	Short: "get a block by block ID",
	Run: func(cmd *cobra.Command, args []string) {
		storages := InitStorages()

		blockID, err := flow.HexStringToIdentifier(flagBlockID)
		if err != nil {
			log.Fatal().Err(err).Msg("malformed block id")
		}

		log.Info().Msgf("getting block by id: %v", blockID)
		block, err := storages.Blocks.ByID(blockID)
		if err != nil {
			log.Fatal().Err(err).Msgf("could not get block with id: %v", blockID)
		}

		common.PrettyPrintEntity(block)
	},
}
