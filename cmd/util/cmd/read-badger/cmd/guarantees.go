package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
)

func init() {
	rootCmd.AddCommand(guaranteesCmd)

	guaranteesCmd.Flags().StringVarP(&flagCollectionID, "collection-id", "c", "", "the collection id of which to query the guarantee")
	_ = guaranteesCmd.MarkFlagRequired("collection-id")
}

var guaranteesCmd = &cobra.Command{
	Use:   "guarantees",
	Short: "get guarantees by collection ID",
	Run: func(cmd *cobra.Command, args []string) {
		storages, db := InitStorages()
		defer db.Close()

		log.Info().Msgf("got flag collection id: %s", flagCollectionID)
		collectionID, err := flow.HexStringToIdentifier(flagCollectionID)
		if err != nil {
			log.Fatal().Err(err).Msg("malformed collection idenitifer")
		}

		log.Info().Msgf("getting guarantee by collection id: %v", collectionID)
		guarantee, err := storages.Guarantees.ByCollectionID(collectionID)
		if err != nil {
			log.Fatal().Err(err).Msgf("could not get guarantee for collection id: %v", collectionID)
		}

		common.PrettyPrintEntity(guarantee)
	},
}
