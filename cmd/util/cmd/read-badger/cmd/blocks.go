package cmd

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	badgerstate "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store"
)

var flagBlockID string
var flagBlockHeight uint64

func init() {
	rootCmd.AddCommand(blocksCmd)

	blocksCmd.Flags().StringVarP(&flagBlockID, "id", "i", "", "the id of the block")
	blocksCmd.Flags().Uint64Var(&flagBlockHeight, "height", 0, "Block height")
}

var blocksCmd = &cobra.Command{
	Use:   "blocks",
	Short: "get a block by block ID or height",
	RunE: func(cmd *cobra.Command, args []string) error {
		return common.WithStorage(flagDatadir, func(db storage.DB) error {
			chainID, err := badgerstate.GetChainID(db)
			if err != nil {
				return err
			}
			cacheMetrics := metrics.NewNoopCollector()
			headers, err := store.NewHeaders(cacheMetrics, db, chainID)
			if err != nil {
				return err
			}
			index := store.NewIndex(cacheMetrics, db)
			guarantees := store.NewGuarantees(cacheMetrics, db, store.DefaultCacheSize, store.DefaultCacheSize)
			seals := store.NewSeals(cacheMetrics, db)
			results := store.NewExecutionResults(cacheMetrics, db)
			receipts := store.NewExecutionReceipts(cacheMetrics, db, results, store.DefaultCacheSize)
			payloads := store.NewPayloads(db, index, guarantees, seals, receipts, results)
			blocks := store.NewBlocks(db, headers, payloads)

			var block *flow.Block

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
				return fmt.Errorf("provide either a --id or --height and not both, (--block-id: %v), (--height: %v)", flagBlockID, flagBlockHeight)
			}

			common.PrettyPrintEntity(block)
			return nil
		})
	},
}
