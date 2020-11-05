package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
)

var flagCollectionID string

func init() {
	rootCmd.AddCommand(collectionsCmd)

	collectionsCmd.Flags().StringVar(&flagCollectionID, "collection-id", "", "the ID of the collection")
	_ = collectionsCmd.MarkFlagRequired("collection-id")
}

var collectionsCmd = &cobra.Command{
	Use:   "collections",
	Short: "get collection by collection ID",
	Run: func(cmd *cobra.Command, args []string) {
		storages := InitStorages()

		collectionID, err := flow.HexStringToIdentifier(flagCollectionID)
		if err != nil {
			log.Fatal().Err(err).Msg("malformed collection ID")
		}

		log.Info().Msgf("getting collections by block id: %v", collectionID)

		collection, err := storages.Collections.ByID(collectionID)
		if err != nil {
			log.Fatal().Err(err).Msgf("could not get collection")
		}

		common.PrettyPrintEntity(collection)
	},
}
