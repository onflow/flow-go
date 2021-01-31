package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
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
		storages, db := InitStorages()
		defer db.Close()

		if flagBlockID != "" {
			log.Info().Msgf("got flag block id: %s", flagBlockID)
			blockID, err := flow.HexStringToIdentifier(flagBlockID)
			if err != nil {
				log.Fatal().Err(err).Msg("malformed block id")
			}

			log.Info().Msgf("getting receipt by block id: %v", blockID)
			receipt, err := storages.Receipts.ByBlockID(blockID)
			if err != nil {
				log.Fatal().Err(err).Msgf("could not get receipt for block id: %v", blockID)
			}

			common.PrettyPrintEntity(receipt)
			return
		}

		if flagReceiptID != "" {
			log.Info().Msgf("got flag receipt id: %s", flagReceiptID)
			receiptID, err := flow.HexStringToIdentifier(flagReceiptID)
			if err != nil {
				log.Fatal().Err(err).Msg("malformed receipt id")
			}

			log.Info().Msgf("getting receipt by id: %v", receiptID)
			receipt, err := storages.Receipts.ByID(receiptID)
			if err != nil {
				log.Fatal().Err(err).Msgf("could not get receipt with id: %v", receiptID)
			}

			common.PrettyPrintEntity(receipt)
			return
		}

		log.Fatal().Msg("missing flags: --block-id or --receipt-id")
	},
}
