package cmd

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	badgerstate "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/storage"
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
			chainID, err := badgerstate.GetChainIDFromLatestFinalizedHeader(db)
			if err != nil {
				return err
			}
			storages := common.InitStorages(db, chainID) // TODO(4204) - header storage not used

			log.Info().Msgf("got flag transaction id: %s", flagTransactionID)
			transactionID, err := flow.HexStringToIdentifier(flagTransactionID)
			if err != nil {
				return fmt.Errorf("malformed transaction id: %w", err)
			}

			log.Info().Msgf("getting transaction by id: %v", transactionID)
			tx, err := storages.Transactions.ByID(transactionID)
			if err != nil {
				return fmt.Errorf("could not get transaction with id: %v: %w", transactionID, err)
			}

			common.PrettyPrintEntity(tx)
			return nil
		})
	},
}
