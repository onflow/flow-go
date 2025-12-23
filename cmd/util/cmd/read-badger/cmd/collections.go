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

var flagCollectionID string
var flagTransactionID string
var flagLightCollection bool

func init() {
	rootCmd.AddCommand(collectionsCmd)

	collectionsCmd.Flags().StringVarP(&flagCollectionID, "id", "i", "", "the id of the collection")
	collectionsCmd.Flags().StringVarP(&flagTransactionID, "transaction-id", "t", "", "the id of the transaction (always retrieves a light collection regardless of the --light flag)")
	collectionsCmd.Flags().BoolVar(&flagLightCollection, "light", false, "whether to return the light collection (only transaction IDs)")
}

var collectionsCmd = &cobra.Command{
	Use:   "collections",
	Short: "get collection by collection or transaction ID",
	RunE: func(cmd *cobra.Command, args []string) error {
		return common.WithStorage(flagDatadir, func(db storage.DB) error {
			transactions := store.NewTransactions(&metrics.NoopCollector{}, db)
			collections := store.NewCollections(db, transactions)

			if flagCollectionID != "" {
				log.Info().Msgf("got flag collection id: %s", flagCollectionID)
				collectionID, err := flow.HexStringToIdentifier(flagCollectionID)
				if err != nil {
					return fmt.Errorf("malformed collection id: %w", err)
				}

				log.Info().Msgf("getting collection by id: %v", collectionID)

				// get only the light collection if specified
				if flagLightCollection {
					light, err := collections.LightByID(collectionID)
					if err != nil {
						return fmt.Errorf("could not get collection with id %v: %w", collectionID, err)
					}
					common.PrettyPrintEntity(light)
					return nil
				}

				// otherwise get the full collection
				fullCollection, err := collections.ByID(collectionID)
				if err != nil {
					return fmt.Errorf("could not get collection: %w", err)
				}
				common.PrettyPrintEntity(fullCollection)
				return nil
			}

			if flagTransactionID != "" {
				log.Info().Msgf("got flag transaction id: %s", flagTransactionID)
				transactionID, err := flow.HexStringToIdentifier(flagTransactionID)
				if err != nil {
					return fmt.Errorf("malformed transaction id, %w", err)
				}

				log.Info().Msgf("getting collections by transaction id: %v", transactionID)
				light, err := collections.LightByTransactionID(transactionID)
				if err != nil {
					return fmt.Errorf("could not get collections for transaction id %v: %w", transactionID, err)
				}

				common.PrettyPrintEntity(light)
				return nil
			}

			return fmt.Errorf("must specify exactly one of --collection-id or --transaction-id")
		})
	},
}
