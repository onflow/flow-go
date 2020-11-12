package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
)

func init() {
	rootCmd.AddCommand(transactionResultsCmd)

	transactionResultsCmd.Flags().StringVar(&flagBlockID, "block-id", "", "the block ID of which to query the transaction result")
	_ = transactionResultsCmd.MarkFlagRequired("block-id")
}

var transactionResultsCmd = &cobra.Command{
	Use:   "transaction-results",
	Short: "get transaction-result by --block-id",
	Run: func(cmd *cobra.Command, args []string) {
		storages := InitStorages()

		blockID, err := flow.HexStringToIdentifier(flagBlockID)
		if err != nil {
			log.Fatal().Err(err).Msg("malformed block ID")
		}

		log.Info().Msgf("getting transaction results by block id: %v", blockID)

		block, err := storages.Blocks.ByID(blockID)
		if err != nil {
			log.Fatal().Err(err).Msg("could not get block by ID: %w")
		}

		txs := make([]flow.Identifier, 0)

		for _, collectionID := range block.Payload.Guarantees {
			collection, err := storages.Collections.ByID(collectionID.CollectionID)
			if err != nil {
				log.Fatal().Err(err).Msgf("could not get collection by ID: %v", collectionID.CollectionID)
			}

			for _, tx := range collection.Transactions {
				txs = append(txs, tx.ID())
			}
		}

		for _, tx := range txs {
			transactionResult, err := storages.TransactionResults.ByBlockIDTransactionID(blockID, tx)
			if err != nil {
				log.Fatal().Err(err).Msgf("could not get transaction result by block ID and transaction ID: %v", tx)
			}
			common.PrettyPrintEntity(transactionResult)
		}

	},
}
