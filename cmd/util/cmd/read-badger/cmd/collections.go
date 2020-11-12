package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
)

var flagCollectionID string
var flagTransactionID string

func init() {
	rootCmd.AddCommand(collectionsCmd)

	collectionsCmd.Flags().StringVarP(&flagCollectionID, "id", "i", "", "the identifier of the collection")
	collectionsCmd.Flags().StringVarP(&flagTransactionID, "transaction-id", "t", "", "the identifier of the transaction")
}

var collectionsCmd = &cobra.Command{
	Use:   "collections",
	Short: "get collection by collection or transaction ID",
	Run: func(cmd *cobra.Command, args []string) {
		storages := InitStorages()

		if flagCollectionID != "" {
			collectionID, err := flow.HexStringToIdentifier(flagCollectionID)
			if err != nil {
				log.Fatal().Err(err).Msg("malformed collection idenitifer")
			}

			log.Info().Msgf("getting collection by id: %v", collectionID)
			collection, err := storages.Collections.ByID(collectionID)
			if err != nil {
				log.Fatal().Err(err).Msgf("could not get collection")
			}

			common.PrettyPrintEntity(collection)
			return
		}

		if flagTransactionID != "" {
			transactionID, err := flow.HexStringToIdentifier(flagTransactionID)
			if err != nil {
				log.Fatal().Err(err).Msg("malformed transaction indentifer")
			}

			log.Info().Msgf("getting collections by transaction id: %v", transactionID)
			collections, err := storages.Collections.LightByTransactionID(transactionID)
			if err != nil {
				log.Fatal().Err(err).Msg("could not get collections")
			}

			common.PrettyPrintEntity(collections)
			return
		}

		log.Fatal().Msg("missing flags --collection-id or --transaction-id")
	},
}
