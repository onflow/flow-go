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

var flagBlockID string
var flagBlockHeight uint64

func init() {
	rootCmd.AddCommand(blocksCmd)

	blocksCmd.Flags().StringVarP(&flagBlockID, "id", "i", "", "the id of the block")
	blocksCmd.Flags().Uint64Var(&flagHeight, "height", 0, "Block height")
}

var blocksCmd = &cobra.Command{
	Use:   "blocks",
	Short: "get a block by block ID or height",
	RunE: func(cmd *cobra.Command, args []string) error {
		return WithStorage(func(db storage.DB) error {
			cacheMetrics := &metrics.NoopCollector{}
			headers := store.NewHeaders(cacheMetrics, db)
			index := store.NewIndex(cacheMetrics, db)
			guarantees := store.NewGuarantees(cacheMetrics, db, store.DefaultCacheSize)
			seals := store.NewSeals(cacheMetrics, db)
			results := store.NewExecutionResults(cacheMetrics, db)
			receipts := store.NewExecutionReceipts(cacheMetrics, db, results, store.DefaultCacheSize)
			payloads := store.NewPayloads(db, index, guarantees, seals, receipts, results)
			blocks := store.NewBlocks(db, headers, payloads)

			var block *flow.Block
			var err error

			if flagBlockID != "" {
				log.Info().Msgf("got flag block id: %s", flagBlockID)
				blockID, err := flow.HexStringToIdentifier(flagBlockID)
				if err != nil {
					return fmt.Errorf("malformed block id: %w", err)
				}

				log.Info().Msgf("getting block by id: %v", blockID)

				block, err = blocks.ByID(blockID)
				if err != nil {
					return fmt.Errorf("could not get block with id %v: %w", blockID, err)
				}
			} else if flagBlockHeight != 0 {
				log.Info().Msgf("got flag block height: %d", flagBlockHeight)

				block, err = blocks.ByHeight(flagBlockHeight)
				if err != nil {
					return fmt.Errorf("could not get block with height %d: %w", flagBlockHeight, err)
				}
			} else {
				return fmt.Errorf("provide either a --id or --height and not both")
			}

			common.PrettyPrintEntity(block)
			return nil
		})
	},
}
