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

	blocksCmd.Flags().StringVar(&flagBlockID, "block-id", "", "the ID of the block")
	_ = blocksCmd.MarkFlagRequired("block-id")
}

var blocksCmd = &cobra.Command{
	Use:   "blocks",
	Short: "get block by block ID",
	Run: func(cmd *cobra.Command, args []string) {
		storages := InitStorages()

		blockID, err := flow.HexStringToIdentifier(flagBlockID)
		if err != nil {
			log.Fatal().Err(err).Msg("malformed block ID")
		}

		log.Info().Msgf("getting blocks by block id: %v", blockID)

		block, err := storages.Blocks.ByID(blockID)
		if err != nil {
			log.Fatal().Err(err).Msgf("could not get block")
		}

		common.PrettyPrintEntity(block)
	},
}
