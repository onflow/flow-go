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

var flagChunkID string

func init() {
	rootCmd.AddCommand(chunkDataPackCmd)

	chunkDataPackCmd.Flags().StringVarP(&flagChunkID, "id", "c", "", "the id of the chunk")
	_ = chunkDataPackCmd.MarkFlagRequired("id")
}

var chunkDataPackCmd = &cobra.Command{
	Use:   "chunk-data-packs",
	Short: "get chunk data pack by chunk ID",
	Run: func(cmd *cobra.Command, args []string) {
		err := WithStorage(func(db storage.DB) error {
			log.Info().Msgf("got flag chunk id: %s", flagChunkID)
			chunkID, err := flow.HexStringToIdentifier(flagChunkID)
			if err != nil {
				return fmt.Errorf("malformed chunk id: %w", err)
			}

			metrics := metrics.NewNoopCollector()
			collections := store.NewCollections(db, store.NewTransactions(metrics, db))
			chunkDataPacks := store.NewChunkDataPacks(metrics,
				db, collections, 1)

			log.Info().Msgf("getting chunk data pack by chunk id: %v", chunkID)
			chunkDataPack, err := chunkDataPacks.ByChunkID(chunkID)
			if err != nil {
				log.Error().Err(err).Msgf("could not get chunk data pack with chunk id: %v", chunkID)
				return nil
			}

			common.PrettyPrintEntity(chunkDataPack)
			return nil
		})

		if err != nil {
			log.Error().Err(err).Msg("could not get chunk data pack")
		}
	},
}
