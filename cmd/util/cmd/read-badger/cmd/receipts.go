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

var flagReceiptID string

func init() {
	rootCmd.AddCommand(receiptsCmd)

	receiptsCmd.Flags().StringVarP(&flagBlockID, "block-id", "b", "", "the block id of which to query the receipt")
	receiptsCmd.Flags().StringVarP(&flagReceiptID, "id", "i", "", "the id of the receipt")
}

var receiptsCmd = &cobra.Command{
	Use:   "receipts",
	Short: "get receipt by block or receipt ID",
	Run: func(cmd *cobra.Command, args []string) {
		err := WithStorage(func(db storage.DB) error {
			results := store.NewExecutionResults(metrics.NewNoopCollector(), db)
			receipts := store.NewExecutionReceipts(metrics.NewNoopCollector(), db, results, 1)

			if flagBlockID != "" {
				log.Info().Msgf("got flag block id: %s", flagBlockID)
				blockID, err := flow.HexStringToIdentifier(flagBlockID)
				if err != nil {
					return fmt.Errorf("malformed block id: %w", err)
				}

				log.Info().Msgf("getting receipt by block id: %v", blockID)
				recs, err := receipts.ByBlockID(blockID)
				if err != nil {
					return fmt.Errorf("could not get receipt for block id %v: %w", blockID, err)
				}

				if len(recs) == 0 {
					log.Info().Msgf("no receipt found")
					return nil
				}

				common.PrettyPrintEntity(recs[0])
				return nil
			}

			if flagReceiptID != "" {
				log.Info().Msgf("got flag receipt id: %s", flagReceiptID)
				receiptID, err := flow.HexStringToIdentifier(flagReceiptID)
				if err != nil {
					return fmt.Errorf("malformed receipt id: %w", err)
				}

				log.Info().Msgf("getting receipt by id: %v", receiptID)
				receipt, err := receipts.ByID(receiptID)
				if err != nil {
					return fmt.Errorf("could not get receipt with id %v: %w", receiptID, err)
				}

				common.PrettyPrintEntity(receipt)
				return nil
			}

			return fmt.Errorf("missing flags: --block-id or --receipt-id")
		})

		if err != nil {
			log.Error().Err(err).Msg("could not get receipts")
		}
	},
}
