package cmd

import (
	"fmt"
	"runtime"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/badgerimpl"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/pebble"
)

var flagDBType string

func init() {
	rootCmd.AddCommand(statsCmd)
	statsCmd.Flags().StringVar(&flagDBType, "dbtype", "badger", "database type to use (badger or pebble)")
}

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "get stats for the database, such as key count, total value size, min/max value size etc",
	RunE: func(cmd *cobra.Command, args []string) error {
		var sdb storage.DB
		if flagDBType == "badger" {
			db := common.InitStorage(flagDatadir)
			defer db.Close()
			sdb = badgerimpl.ToDB(db)
		} else if flagDBType == "pebble" {
			pdb, err := pebble.MustOpenDefaultPebbleDB(log.Logger, flagPebbleDir)
			if err != nil {
				return fmt.Errorf("failed to open pebble db: %w", err)
			}
			defer pdb.Close()
			sdb = pebbleimpl.ToDB(pdb)
		} else {
			return fmt.Errorf("invalid db type")
		}

		numWorkers := runtime.NumCPU()
		if numWorkers > 256 {
			numWorkers = 256
		}
		log.Info().Msgf("getting stats for %s db at %s with %v workers0", flagDBType, flagDatadir, numWorkers)
		stats, err := operation.SummarizeKeysByFirstByteConcurrent(log.Logger, sdb.Reader(), numWorkers)
		if err != nil {
			return fmt.Errorf("failed to get stats: %w", err)
		}

		operation.PrintStats(log.Logger, stats)
		return nil
	},
}
