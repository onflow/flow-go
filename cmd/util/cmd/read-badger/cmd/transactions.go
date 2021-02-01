package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
)

func init() {
	rootCmd.AddCommand(transactionsCmd)

	transactionsCmd.Flags().StringVarP(&flagTransactionID, "id", "i", "", "the id of the transaction")
	_ = transactionsCmd.MarkFlagRequired("id")
}

var transactionsCmd = &cobra.Command{
	Use:   "transactions",
	Short: "get transaction by ID",
	Run: func(cmd *cobra.Command, args []string) {
		storages, db := InitStorages()
		defer db.Close()

		log.Info().Msgf("got flag transaction id: %s", flagTransactionID)
		transactionID, err := flow.HexStringToIdentifier(flagTransactionID)
		if err != nil {
			log.Error().Err(err).Msg("malformed transaction id")
			return
		}

		log.Info().Msgf("getting transaction by id: %v", transactionID)
		tx, err := storages.Transactions.ByID(transactionID)
		if err != nil {
			log.Error().Err(err).Msgf("could not get transaction with id: %v", transactionID)
			return
		}

		common.PrettyPrintEntity(tx)
	},
}
