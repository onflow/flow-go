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

func init() {
	rootCmd.AddCommand(transactionResultsCmd)

	transactionResultsCmd.Flags().StringVarP(&flagBlockID, "block-id", "b", "", "the block id of which to query the transaction result")
	_ = transactionResultsCmd.MarkFlagRequired("block-id")
}

var transactionResultsCmd = &cobra.Command{
	Use:   "transaction-results",
	Short: "get transaction-result by block ID",
	RunE: func(cmd *cobra.Command, args []string) error {
		return common.WithStorage(flagDatadir, func(db storage.DB) error {
			transactionResults, err := store.NewTransactionResults(metrics.NewNoopCollector(), db, 1)
			if err != nil {
				return err
			}
			storages := common.InitStorages(db)
			log.Info().Msgf("got flag block id: %s", flagBlockID)
			blockID, err := flow.HexStringToIdentifier(flagBlockID)
			if err != nil {
				return fmt.Errorf("malformed block id: %w", err)
			}

			log.Info().Msgf("getting transaction results by block id: %v", blockID)
			block, err := storages.Blocks.ByID(blockID)
			if err != nil {
				return fmt.Errorf("could not get block with id: %w", err)
			}

			log.Info().Msgf("getting transaction results by block id: %v", blockID)

			txIDs := make([]flow.Identifier, 0)

			for _, guarantee := range block.Payload.Guarantees {
				collection, err := storages.Collections.ByID(guarantee.CollectionID)
				if err != nil {
					return fmt.Errorf("could not get collection with id %v: %w", guarantee.CollectionID, err)
				}
				for _, tx := range collection.Transactions {
					txIDs = append(txIDs, tx.ID())
				}
			}

			for _, txID := range txIDs {
				transactionResult, err := transactionResults.ByBlockIDTransactionID(blockID, txID)
				if err != nil {
					return fmt.Errorf("could not get transaction result for block id and transaction id: %v: %w", txID, err)
				}
				common.PrettyPrint(transactionResult)
			}

			return nil
		})
	},
}
