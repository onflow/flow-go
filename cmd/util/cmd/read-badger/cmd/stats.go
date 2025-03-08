package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/badgerimpl"
)

var flagDBType string
var flagWorker int

func init() {
	rootCmd.AddCommand(statsCmd)
	statsCmd.Flags().StringVar(&flagDBType, "dbtype", "badger", "database type to use (badger or pebble)")
	statsCmd.Flags().IntVar(&flagWorker, "worker", 10, "number of workers to use")
}

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "get stats for the database",
	Run: func(cmd *cobra.Command, args []string) {
		db := common.InitStorage(flagDatadir)
		defer db.Close()

		var sdb storage.DB
		if flagDBType == "badger" {
			sdb = badgerimpl.ToDB(db)
		} else if flagDBType == "pebble" {
			// TODO: implement pebble
			log.Error().Msg("pebble not implemented")
		} else {
			log.Error().Msg("invalid db type")
			return
		}

		log.Info().Msgf("getting stats for %s db at %s with %v worker", flagDBType, flagDatadir, flagWorker)
		stats, err := operation.SummarizeKeysByFirstByteConcurrent(log.Logger, sdb.Reader(), flagWorker)
		if err != nil {
			log.Error().Err(err).Msg("failed to get stats")
		}

		operation.PrintStats(log.Logger, stats)
	},
}
