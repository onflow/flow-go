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

	collectionsCmd.Flags().StringVar(&flagCollectionID, "collection-id", "", "the ID of the collection")
	collectionsCmd.Flags().StringVar(&flagTransactionID, "transaction-id", "", "the ID of the transaction")
}

var collectionsCmd = &cobra.Command{
	Use:   "collections",
	Short: "get collection by collection ID (--colleciton-id) or transaction ID (--transaction-id)",
	Run: func(cmd *cobra.Command, args []string) {
		storages := InitStorages()

		if flagCollectionID != "" {
			log.Info().Msgf("flag collection id: %v", flagCollectionID)
			collectionID, err := flow.HexStringToIdentifier(flagCollectionID)
			if err != nil {
				log.Fatal().Err(err).Msg("malformed collection ID")
			}

			log.Info().Msgf("getting collections by collection id: %v", collectionID)

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
				log.Fatal().Err(err).Msg("malformed transaction ID")
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
