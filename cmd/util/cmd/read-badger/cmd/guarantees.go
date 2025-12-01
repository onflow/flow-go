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
	rootCmd.AddCommand(guaranteesCmd)

	guaranteesCmd.Flags().StringVarP(&flagCollectionID, "collection-id", "c", "", "the collection id of which to query the guarantee")
	_ = guaranteesCmd.MarkFlagRequired("collection-id")
}

var guaranteesCmd = &cobra.Command{
	Use:   "guarantees",
	Short: "get guarantees by collection ID",
	RunE: func(cmd *cobra.Command, args []string) error {
		return common.WithStorage(flagDatadir, func(db storage.DB) error {
			chainID, err := badgerstate.GetChainIDFromLatestFinalizedHeader(db)
			if err != nil {
				return err
			}
			storages := common.InitStorages(db, chainID) // TODO(4204) - header storage not used

			log.Info().Msgf("got flag collection id: %s", flagCollectionID)
			collectionID, err := flow.HexStringToIdentifier(flagCollectionID)
			if err != nil {
				return fmt.Errorf("malformed collection identifier: %w", err)
			}

			log.Info().Msgf("getting guarantee by collection id: %v", collectionID)
			guarantee, err := storages.Guarantees.ByCollectionID(collectionID)
			if err != nil {
				return fmt.Errorf("could not get guarantee for collection id: %v: %w", collectionID, err)
			}

			common.PrettyPrintEntity(guarantee)
			return nil
		})
	},
}
