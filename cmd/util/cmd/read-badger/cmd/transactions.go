package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
)

func init() {
	rootCmd.AddCommand(transactionsCmd)

	transactionsCmd.Flags().StringVar(&flagTransactionID, "id", "", "the transaction ID of the transaction")
	_ = transactionsCmd.MarkFlagRequired("id")
}

var transactionsCmd = &cobra.Command{
	Use:   "transactions",
	Short: "get transaction by --id",
	Run: func(cmd *cobra.Command, args []string) {
		storages := InitStorages()

		transactionID, err := flow.HexStringToIdentifier(flagTransactionID)
		if err != nil {
			log.Fatal().Err(err).Msg("malformed transaction ID")
		}

		log.Info().Msgf("getting transaction by id: %v", transactionID)

		tx, err := storages.Transactions.ByID(transactionID)
		if err != nil {
			log.Fatal().Err(err).Msg("could not get transaction by ID")
		}

		common.PrettyPrintEntity(tx)
	},
}
