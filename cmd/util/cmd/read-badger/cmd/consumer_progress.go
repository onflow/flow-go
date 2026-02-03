package cmd

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

var (
	flagConsumerProgressID  string
	flagConsumerProgressAll bool
)

// allConsumerProgressIDs contains all known consumer progress IDs defined in module/jobqueue.go
var allConsumerProgressIDs = []string{
	module.ConsumeProgressVerificationBlockHeight,
	module.ConsumeProgressVerificationChunkIndex,
	module.ConsumeProgressExecutionDataRequesterBlockHeight,
	module.ConsumeProgressExecutionDataRequesterNotification,
	module.ConsumeProgressExecutionDataIndexerBlockHeight,
	module.ConsumeProgressIngestionEngineBlockHeight,
	module.ConsumeProgressEngineTxErrorMessagesBlockHeight,
	module.ConsumeProgressLastFullBlockHeight,
}

func init() {
	rootCmd.AddCommand(consumerProgressCmd)

	consumerProgressCmd.Flags().StringVarP(&flagConsumerProgressID, "id", "i", "", "query a specific consumer progress id (e.g., ConsumeProgressVerificationBlockHeight)")
	consumerProgressCmd.Flags().BoolVar(&flagConsumerProgressAll, "all", false, "query all known consumer progress IDs (default behavior)")
}

var consumerProgressCmd = &cobra.Command{
	Use:   "consumer-progress",
	Short: "get the processed height by consumer progress id",
	RunE: func(cmd *cobra.Command, args []string) error {
		return common.WithStorage(flagDatadir, func(db storage.DB) error {
			if flagConsumerProgressAll && flagConsumerProgressID != "" {
				return fmt.Errorf("cannot use both --id and --all flags")
			}

			// If --id is specified, query single consumer progress
			if flagConsumerProgressID != "" {
				return querySingleConsumerProgress(db, flagConsumerProgressID)
			}

			// Default: query all consumer progress
			return queryAllConsumerProgress(db)
		})
	},
}

func querySingleConsumerProgress(db storage.DB, progressID string) error {
	log.Info().Msgf("reading consumer progress for id: %s", progressID)

	var processedIndex uint64
	err := operation.RetrieveProcessedIndex(db.Reader(), progressID, &processedIndex)
	if err != nil {
		return fmt.Errorf("could not retrieve processed index for consumer %s: %w", progressID, err)
	}

	fmt.Printf("Consumer Progress ID: %s\n", progressID)
	fmt.Printf("Processed Height: %d\n", processedIndex)

	return nil
}

func queryAllConsumerProgress(db storage.DB) error {
	log.Info().Msg("reading all consumer progress")

	fmt.Printf("%-60s %s\n", "Consumer Progress ID", "Processed Height")
	fmt.Printf("%-60s %s\n", "--------------------", "----------------")

	for _, progressID := range allConsumerProgressIDs {
		var processedIndex uint64
		err := operation.RetrieveProcessedIndex(db.Reader(), progressID, &processedIndex)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				fmt.Printf("%-60s %s\n", progressID, "not found")
				continue
			}
			return fmt.Errorf("could not retrieve processed index for consumer %s: %w", progressID, err)
		}

		fmt.Printf("%-60s %d\n", progressID, processedIndex)
	}

	return nil
}
