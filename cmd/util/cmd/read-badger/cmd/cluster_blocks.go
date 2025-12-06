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

var flagChainName string
var flagClusterBlockID string
var flagHeight uint64

func init() {
	rootCmd.AddCommand(clusterBlocksCmd)

	clusterBlocksCmd.Flags().StringVarP(&flagChainName, "chain", "c", "", "the name of the chain")
	_ = clusterBlocksCmd.MarkFlagRequired("chain")

	clusterBlocksCmd.Flags().StringVarP(&flagClusterBlockID, "id", "i", "", "the id of the cluster block")
	clusterBlocksCmd.Flags().Uint64VarP(&flagHeight, "height", "h", 0, "the height of the cluster block")
}

var clusterBlocksCmd = &cobra.Command{
	Use:   "cluster-blocks",
	Short: "get cluster blocks",
	RunE: func(cmd *cobra.Command, args []string) error {
		return common.WithStorage(flagDatadir, func(db storage.DB) error {
			metrics := metrics.NewNoopCollector()

			// get chain id
			log.Info().Msgf("got flag chain name: %s", flagChainName)
			chainID := flow.ChainID(flagChainName)

			headers := store.NewHeaders(metrics, db, chainID)
			clusterPayloads := store.NewClusterPayloads(metrics, db)
			clusterBlocks := store.NewClusterBlocks(db, chainID, headers, clusterPayloads)

			if flagClusterBlockID != "" && flagHeight != 0 {
				return fmt.Errorf("provide either a --id or --height and not both")
			}

			if flagClusterBlockID != "" {
				log.Info().Msgf("got flag cluster block id: %s", flagClusterBlockID)
				clusterBlockID, err := flow.HexStringToIdentifier(flagClusterBlockID)
				if err != nil {
					return fmt.Errorf("malformed cluster block id: %w", err)
				}

				log.Info().Msgf("getting cluster block by id: %v", clusterBlockID)
				clusterBlock, err := clusterBlocks.ProposalByID(clusterBlockID)
				if err != nil {
					return fmt.Errorf("could not get cluster block with id: %v, %w", clusterBlockID, err)
				}

				common.PrettyPrint(clusterBlock)
				return nil
			}

			if flagHeight > 0 {
				log.Info().Msgf("getting cluster block by height: %v", flagHeight)
				clusterBlock, err := clusterBlocks.ProposalByHeight(flagHeight)
				if err != nil {
					return fmt.Errorf("could not get cluster block with height: %v, %w", flagHeight, err)
				}

				log.Info().Msgf("block id: %v", clusterBlock.Block.ID())
				common.PrettyPrint(clusterBlock)
				return nil
			}

			return fmt.Errorf("provide either a --id or --height")
		})
	},
}
