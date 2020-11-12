package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
)

func init() {
	rootCmd.AddCommand(transactionResultsCmd)

	transactionResultsCmd.Flags().StringVarP(&flagBlockID, "block-id", "b", "", "the block ID of which to query the transaction result")
	_ = transactionResultsCmd.MarkFlagRequired("block-id")
}

var transactionResultsCmd = &cobra.Command{
	Use:   "transaction-results",
	Short: "get transaction-result by block ID",
	Run: func(cmd *cobra.Command, args []string) {
		storages := InitStorages()

		blockID, err := flow.HexStringToIdentifier(flagBlockID)
		if err != nil {
			log.Fatal().Err(err).Msg("malformed block identifier")
		}

		log.Info().Msgf("getting transaction results by block id: %v", blockID)

		block, err := storages.Blocks.ByID(blockID)
		if err != nil {
			log.Fatal().Err(err).Msg("could not get block with identifier: %w")
		}

		txIDs := make([]flow.Identifier, 0)

		for _, guarantee := range block.Payload.Guarantees {
			collection, err := storages.Collections.ByID(guarantee.CollectionID)
			if err != nil {
				log.Fatal().Err(err).Msgf("could not get collection with identifier: %v", guarantee.CollectionID)
			}

			for _, tx := range collection.Transactions {
				txIDs = append(txIDs, tx.ID())
			}
		}

		for _, txID := range txIDs {
			transactionResult, err := storages.TransactionResults.ByBlockIDTransactionID(blockID, txID)
			if err != nil {
				log.Fatal().Err(err).Msgf("could not get transaction result with block ID and transaction ID: %v", txID)
			}
			common.PrettyPrintEntity(transactionResult)
		}
	},
}
