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
	rootCmd.AddCommand(transactionsCmd)

	transactionsCmd.Flags().StringVarP(&flagTransactionID, "id", "i", "", "the id of the transaction")
	_ = transactionsCmd.MarkFlagRequired("id")
}

var transactionsCmd = &cobra.Command{
	Use:   "transactions",
	Short: "get transaction by ID",
	RunE: func(cmd *cobra.Command, args []string) error {
		return common.WithStorage(flagDatadir, func(db storage.DB) error {
			transactions := store.NewTransactions(&metrics.NoopCollector{}, db)

			log.Info().Msgf("got flag transaction id: %s", flagTransactionID)
			transactionID, err := flow.HexStringToIdentifier(flagTransactionID)
			if err != nil {
				return fmt.Errorf("malformed transaction id: %w", err)
			}

			log.Info().Msgf("getting transaction by id: %v", transactionID)
			tx, err := transactions.ByID(transactionID)
			if err != nil {
				return fmt.Errorf("could not get transaction with id: %v: %w", transactionID, err)
			}

			common.PrettyPrintEntity(tx)
			return nil
		})
	},
}
