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

	receiptsCmd.Flags().StringVar(&flagBlockID, "block-id", "", "the block ID of which to query the receipt")
	receiptsCmd.Flags().StringVar(&flagReceiptID, "receipt-id", "", "the ID of the receipt")
}

var receiptsCmd = &cobra.Command{
	Use:   "receipts",
	Short: "get receipt by --block-id or --receipt-id",
	Run: func(cmd *cobra.Command, args []string) {
		storages := InitStorages()

		if flagBlockID != "" {
			blockID, err := flow.HexStringToIdentifier(flagBlockID)
			if err != nil {
				log.Fatal().Err(err).Msg("malformed block ID")
			}

			log.Info().Msgf("getting receipt by block id: %v", blockID)

			receipt, err := storages.Receipts.ByBlockID(blockID)
			if err != nil {
				log.Fatal().Err(err).Msgf("could not get receipt")
			}

			common.PrettyPrintEntity(receipt)
			return
		}

		if flagReceiptID != "" {
			receiptID, err := flow.HexStringToIdentifier(flagReceiptID)
			if err != nil {
				log.Fatal().Err(err).Msg("malformed receipt ID")
			}

			log.Info().Msgf("getting receipts by receipt id: %v", receiptID)

			receipt, err := storages.Receipts.ByID(receiptID)
			if err != nil {
				log.Fatal().Err(err).Msgf("could not get receipt")
			}

			common.PrettyPrintEntity(receipt)
			return
		}

		log.Fatal().Msg("missing flags: --block-id or --receipt-id")
	},
}
