package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
)

var flagCollectionID string
var flagTransactionID string
var flagLightCollection bool

func init() {
	rootCmd.AddCommand(collectionsCmd)

	collectionsCmd.Flags().StringVarP(&flagCollectionID, "id", "i", "", "the id of the collection")
	collectionsCmd.Flags().StringVarP(&flagTransactionID, "transaction-id", "t", "", "the id of the transaction (always retrieves a light collection regardless of the --light flag)")
	collectionsCmd.Flags().BoolVarP(&flagLightCollection, "light", "l", false, "whether to return the light collection (only transaction IDs)")
}

var collectionsCmd = &cobra.Command{
	Use:   "collections",
	Short: "get collection by collection or transaction ID",
	Run: func(cmd *cobra.Command, args []string) {
		storages, db := InitStorages()
		defer db.Close()

		if flagCollectionID != "" {
			log.Info().Msgf("got flag collection id: %s", flagCollectionID)
			collectionID, err := flow.HexStringToIdentifier(flagCollectionID)
			if err != nil {
				log.Error().Err(err).Msg("malformed collection id")
				return
			}

			log.Info().Msgf("getting collection by id: %v", collectionID)

			// get only the light collection if specified
			if flagLightCollection {
				light, err := storages.Collections.LightByID(collectionID)
				if err != nil {
					log.Error().Err(err).Msgf("could not get collection with id: %v", collectionID)
					return
				}
				common.PrettyPrintEntity(light)
				return
			}

			// otherwise get the full collection
			fullCollection, err := storages.Collections.ByID(collectionID)
			if err != nil {
				log.Error().Err(err).Msgf("could not get collection ")
				return
			}
			common.PrettyPrintEntity(fullCollection)
			return
		}

		if flagTransactionID != "" {
			log.Info().Msgf("got flag transaction id: %s", flagTransactionID)
			transactionID, err := flow.HexStringToIdentifier(flagTransactionID)
			if err != nil {
				log.Error().Err(err).Msg("malformed transaction id")
				return
			}

			log.Info().Msgf("getting collections by transaction id: %v", transactionID)
			light, err := storages.Collections.LightByTransactionID(transactionID)
			if err != nil {
				log.Error().Err(err).Msgf("could not get collections for transaction id: %v", transactionID)
				return
			}

			common.PrettyPrintEntity(light)
			return
		}

		log.Error().Msg("must specify exactly one of --collection-id or --transaction-id")
		return
	},
}
