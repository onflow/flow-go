package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
)

func init() {
	rootCmd.AddCommand(resultsCmd)
}

var resultsCmd = &cobra.Command{
	Use:   "results",
	Short: "reindex sealed result IDs by block ID",
	Run: func(cmd *cobra.Command, args []string) {
		db := common.InitStorage(flagDatadir)
		defer db.Close()
		storages := common.InitStorages(db)
		state, err := common.InitProtocolState(db, storages)
		if err != nil {
			log.Fatal().Err(err).Msg("could not init protocol state")
		}

		results := storages.Results
		blocks := storages.Blocks

		root := state.Params().FinalizedRoot()
		final, err := state.Final().Head()
		if err != nil {
			log.Fatal().Err(err).Msg("could not get final header from protocol state")
		}

		for h := root.Height + 1; h <= final.Height; h++ {
			block, err := blocks.ByHeight(h)
			if err != nil {
				log.Fatal().Err(err).Msgf("could not get block at height %d", h)
			}

			for _, seal := range block.Payload.Seals {
				err := results.Index(seal.BlockID, seal.ResultID)
				if err != nil {
					log.Fatal().Err(err).Msgf("could not index result ID at height %d", h)
				}
			}
		}

		log.Info().Uint64("start_height", root.Height).Uint64("end_height", final.Height).Msg("indexed execution results")
	},
}
