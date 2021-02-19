package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
)

func init() {
	rootCmd.AddCommand(lightCollectionsCmd)

	lightCollectionsCmd.Flags().StringVarP(&flagCollectionID, "id", "i", "", "the id of the collection")
	_ = transactionsCmd.MarkFlagRequired("id")
}

var lightCollectionsCmd = &cobra.Command{
	Use:   "lightcollections",
	Short: "get light collection by ID",
	Run: func(cmd *cobra.Command, args []string) {
		storages, db := InitStorages()
		defer db.Close()

		log.Info().Msgf("got flag collection id: %s", flagCollectionID)
		collectionID, err := flow.HexStringToIdentifier(flagCollectionID)
		if err != nil {
			log.Error().Err(err).Msg("malformed collection id")
			return
		}

		log.Info().Msgf("getting collection by id: %v", collectionID)
		collection, err := storages.Collections.LightByID(collectionID)
		if err != nil {
			log.Error().Err(err).Msgf("could not get transaction with id: %v", collectionID)
			return
		}

		common.PrettyPrintEntity(collection)
	},
}
