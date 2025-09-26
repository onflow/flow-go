package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/util"
)

var (
	flagStartHeight uint64
	flagEndHeight   uint64
)

func init() {
	rootCmd.PersistentFlags().Uint64Var(&flagStartHeight, "start-height", 0, "start height to backfill from")
	rootCmd.PersistentFlags().Uint64Var(&flagEndHeight, "end-height", 0, "end height to backfill to")
}

func init() {
	rootCmd.AddCommand(collectionsCmd)
}

// this command reindexes collections and sealed results by block ID. This is typically performed by
// the AN's ingestion engine. Before mid-mainnet24, the ingestion engine did not use a jobqueue to
// ensure all blocks were indexed. This command can be used to fix any blocks with missing data.
var collectionsCmd = &cobra.Command{
	Use:   "collections",
	Short: "reindex collections and sealed results by block ID",
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

		root := state.Params().SealedRoot()
		final, err := state.Final().Head()
		if err != nil {
			log.Fatal().Err(err).Msg("could not get final header from protocol state")
		}

		startHeight := root.Height + 1
		if flagStartHeight > 0 {
			if flagStartHeight <= root.Height {
				log.Fatal().Msgf("start height must be greater than root height %d", root.Height)
			}
			startHeight = flagStartHeight
		}

		endHeight := final.Height
		if flagEndHeight > 0 {
			if flagEndHeight > final.Height {
				log.Fatal().Msgf("end height must be less than or equal to final height %d", final.Height)
			}
			endHeight = flagEndHeight
		}

		progress := util.LogProgress(log.Logger,
			util.DefaultLogProgressConfig("backfilling", int(endHeight-startHeight+1)),
		)

		for h := startHeight; h <= endHeight; h++ {
			block, err := blocks.ByHeight(h)
			if err != nil {
				log.Fatal().Err(err).Msgf("could not get block at height %d", h)
			}

			for _, seal := range block.Payload.Seals {
				err := results.Index(seal.BlockID, seal.ResultID)
				if err != nil {
					log.Fatal().Err(err).Msg("could not index block for execution result")
				}
			}

			err = blocks.IndexBlockForCollections(block.Header.ID(), flow.GetIDs(block.Payload.Guarantees))
			if err != nil {
				log.Fatal().Err(err).Msg("could not index block for collections")
			}

			progress(1)
		}

		log.Info().Uint64("start_height", startHeight).Uint64("end_height", endHeight).Msg("indexed collections and sealed results")
	},
}
