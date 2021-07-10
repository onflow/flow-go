package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
)

var flagBlockID string
var flagNParents int

func init() {
	rootCmd.AddCommand(blocksCmd)

	blocksCmd.Flags().StringVarP(&flagBlockID, "id", "i", "", "the id of the block")
	_ = blocksCmd.MarkFlagRequired("id")

	blocksCmd.Flags().IntVarP(&flagNParents, "parents", "p", 0, "the number of parent blocks to fetch")
}

var blocksCmd = &cobra.Command{
	Use:   "blocks",
	Short: "get a block by block ID",
	Run: func(cmd *cobra.Command, args []string) {
		storages, db := InitStorages()
		defer db.Close()

		log.Info().Msgf("got flag block id: %s", flagBlockID)
		blockID, err := flow.HexStringToIdentifier(flagBlockID)
		if err != nil {
			log.Error().Err(err).Msg("malformed block id")
			return
		}

		log.Info().Msgf("getting block by id: %v", blockID)
		block, err := storages.Blocks.ByID(blockID)
		if err != nil {
			log.Error().Err(err).Msgf("could not get block with id: %v", blockID)
			return
		}
		common.PrettyPrintEntity(block)

		parentID := block.Header.ParentID
		n := flagNParents
		for ; n > 0; n-- {
			log.Info().Msgf("getting parent block by id: %v (%v/%v)", parentID, flagNParents-n+1, flagNParents)
			parent, err := storages.Blocks.ByID(parentID)
			if err != nil {
				log.Fatal().Err(err).Msgf("could not get parent with id: %v", parentID)
			}
			common.PrettyPrintEntity(parent)
			parentID = parent.Header.ParentID
		}

	},
}
