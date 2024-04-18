package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/module/util"
)

var flagStartHeight uint64
var flagEndHeight uint64

func init() {
	rootCmd.AddCommand(findEmptyEvents)

	findEmptyEvents.Flags().Uint64VarP(&flagStartHeight, "start-height", "s", 0, "the start height")
	findEmptyEvents.Flags().Uint64VarP(&flagEndHeight, "end-height", "e", 0, "the end height")
}

var findEmptyEvents = &cobra.Command{
	Use:   "find-missing-events",
	Short: "Read events from badger",
	Run: func(cmd *cobra.Command, args []string) {
		storages, db := InitStorages()
		defer db.Close()

		if flagEventType != "" && flagTransactionID != "" {
			log.Error().Msg("provide only one of --transaction-id or --event-type")
			return
		}

		log.Info().Msgf("got flag height range [%v,%v]", flagStartHeight, flagEndHeight)

		if flagEndHeight < flagStartHeight {
			log.Fatal().Msgf("end height %v is less than start height %v", flagEndHeight, flagStartHeight)
		}

		logging := util.LogProgress(
			log.Logger,
			util.DefaultLogProgressConfig(
				"reading events by height",
				int(flagEndHeight-flagStartHeight),
			),
		)
		for height := flagStartHeight; height <= flagEndHeight; height++ {
			logging(1)
			// just fetch events for block
			blockID, err := storages.Headers.BlockIDByHeight(height)
			if err != nil {
				log.Error().Err(err).Msgf("could not get block id for height: %v", height)
				return
			}

			events, err := storages.Events.ByBlockID(blockID)
			if err != nil {
				log.Error().Err(err).Msgf("could not get events for block id: %v", blockID)
				return
			}

			if len(events) == 0 {
				log.Info().Msgf("no events found for block height: %v", blockID)
				return
			}
		}
	},
}
