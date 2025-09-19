package cmd

import (
	"fmt"
	"runtime"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

func init() {
	rootCmd.AddCommand(statsCmd)
}

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "get stats for the database, such as key count, total value size, min/max value size etc",
	RunE: func(cmd *cobra.Command, args []string) error {
		return common.WithStorage(flagDatadir, func(sdb storage.DB) error {

			numWorkers := runtime.NumCPU()
			if numWorkers > 256 {
				numWorkers = 256
			}
			log.Info().Msgf("getting stats with %v workers", numWorkers)

			stats, err := operation.SummarizeKeysByFirstByteConcurrent(log.Logger, sdb.Reader(), numWorkers)
			if err != nil {
				return fmt.Errorf("failed to get stats: %w", err)
			}

			operation.PrintStats(log.Logger, stats)
			return nil
		})
	},
}
