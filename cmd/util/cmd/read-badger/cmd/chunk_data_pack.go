package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
)

var flagChunkID string

func init() {
	rootCmd.AddCommand(chunkDataPackCmd)

	chunkDataPackCmd.Flags().StringVarP(&flagChunkID, "chunk-id", "c", "", "the id of the chunk")
	_ = chunkDataPackCmd.MarkFlagRequired("chunk-id")
}

var chunkDataPackCmd = &cobra.Command{
	Use:   "chunk-data",
	Short: "get chunk data pack by chunk ID",
	Run: func(cmd *cobra.Command, args []string) {
		storages, db := InitStorages()
		defer db.Close()

		log.Info().Msgf("got flag chunk id: %s", flagChunkID)
		chunkID, err := flow.HexStringToIdentifier(flagChunkID)
		if err != nil {
			log.Fatal().Err(err).Msg("malformed chunk id")
		}

		log.Info().Msgf("getting chunk data pack by chunk id: %v", chunkID)
		chunkDataPack, err := storages.ChunkDataPacks.ByChunkID(chunkID)
		if err != nil {
			log.Fatal().Err(err).Msgf("could not get chunk data pack with chunk id: %v", chunkID)
		}

		common.PrettyPrintEntity(chunkDataPack)
	},
}
