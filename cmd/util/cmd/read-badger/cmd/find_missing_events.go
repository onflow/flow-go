package cmd

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/storage"
)

var flagStartHeight uint64
var flagEndHeight uint64
var flagDataDir2 string

func init() {
	rootCmd.AddCommand(findEmptyEvents)

	findEmptyEvents.Flags().Uint64VarP(&flagStartHeight, "start-height", "s", 0, "the start height")
	findEmptyEvents.Flags().Uint64VarP(&flagEndHeight, "end-height", "e", 0, "the end height")
	findEmptyEvents.Flags().StringVarP(&flagDataDir2, "datadir2", "", "", "data dir 2")
}

func initStorages(dir string) (*storage.All, *badger.DB) {
	db := common.InitStorage(dir)
	storages := common.InitStorages(db)
	return storages, db
}

var findEmptyEvents = &cobra.Command{
	Use:   "find-missing-events",
	Short: "Read events from badger",
	Run: func(cmd *cobra.Command, args []string) {
		log.Info().Msgf("got flag height range [%v,%v]", flagStartHeight, flagEndHeight)
		log.Info().Msgf("got flag dir, dir2: %v, %v", flagDatadir, flagDataDir2)

		if flagEndHeight < flagStartHeight {
			log.Fatal().Msgf("end height %v is less than start height %v", flagEndHeight, flagStartHeight)
		}
		if flagDataDir2 == "" || flagDatadir == "" {
			log.Fatal().Msg("datadir and datadir2 must be set")
		}

		log.Info().Msg("initializing storages")
		storages, db := initStorages(flagDatadir)
		defer db.Close()

		log.Info().Msg("initializing storages2")
		storages2, db2 := initStorages(flagDataDir2)
		defer db2.Close()

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
			blockID, err := storages2.Headers.BlockIDByHeight(height)
			if err != nil {
				log.Error().Err(err).Msgf("could not get block id for height: %v", height)
				continue
			}

			events, err := storages.Events.ByBlockID(blockID)
			if err != nil {
				log.Error().Err(err).Msgf("could not get events for block id: %v", blockID)
				continue
			}

			if len(events) == 0 {
				log.Info().Msgf("no events found for block height %v", height)
				events2, err := storages2.Events.ByBlockID(blockID)
				if err != nil {
					log.Error().Err(err).Msgf("could not get events fro block id from storages2: %v", blockID)
					continue
				}

				if len(events2) > 0 {
					log.Info().Msgf("mismatching events for block height %v, actual events: %v", height, len(events2))
				}
			}
		}
	},
}
