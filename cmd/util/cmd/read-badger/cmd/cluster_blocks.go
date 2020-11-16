package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger"
)

var flagChainName string
var flagClusterBlockID string
var flagHeight uint64

func init() {
	rootCmd.AddCommand(clusterBlocksCmd)

	clusterBlocksCmd.Flags().StringVarP(&flagChainName, "chain", "c", "", "the name of the chain")
	_ = clusterBlocksCmd.MarkFlagRequired("chain")

	clusterBlocksCmd.Flags().StringVarP(&flagClusterBlockID, "id", "i", "", "the id of the cluster block")
	clusterBlocksCmd.Flags().Uint64VarP(&flagHeight, "height", "h", 0, "the height of the cluste rblock")
}

var clusterBlocksCmd = &cobra.Command{
	Use:   "clusterblocks",
	Short: "get cluster blocks",
	Run: func(cmd *cobra.Command, args []string) {
		metrics := metrics.NewNoopCollector()
		db := common.InitStorage(flagDatadir)
		headers := badger.NewHeaders(metrics, db)
		clusterPayloads := badger.NewClusterPayloads(metrics, db)

		// get chain id
		chainID := flow.ChainID(flagChainName)
		clusterBlocks := badger.NewClusterBlocks(db, chainID, headers, clusterPayloads)

		if flagClusterBlockID != "" && flagHeight != 0 {
			log.Fatal().Msg("provide either a --id or --height and not both")
			return
		}

		if flagClusterBlockID != "" {
			clusterBlockID, err := flow.HexStringToIdentifier(flagClusterBlockID)
			if err != nil {
				log.Fatal().Err(err).Msg("malformed cluster block id")
			}

			log.Info().Msgf("getting ckuster block by id: %v", clusterBlockID)
			clusterBlock, err := clusterBlocks.ByID(clusterBlockID)
			if err != nil {
				log.Fatal().Err(err).Msgf("could not get cluster block with id: %v", clusterBlockID)
			}

			log.Info().Msgf("block id: %v", clusterBlock.ID())
			common.PrettyPrint(clusterBlock)
			return
		}

		if flagClusterBlockID != "" {
			log.Info().Msgf("getting ckuster block by height: %v", flagHeight)
			clusterBlock, err := clusterBlocks.ByHeight(flagHeight)
			if err != nil {
				log.Fatal().Err(err).Msgf("could not get cluster block with height: %v", flagHeight)
			}

			log.Info().Msgf("block id: %v", clusterBlock.ID())
			log.Info().Msgf("block height: %d", flagHeight)
			common.PrettyPrint(clusterBlock)
			return
		}

		log.Fatal().Msg("provide either a --id or --height")
	},
}
