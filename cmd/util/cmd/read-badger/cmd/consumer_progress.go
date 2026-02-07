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
	flagConsumerProgressID     string
	flagConsumerProgressAll    bool
	flagConsumerProgressHeight uint64
	flagConsumerProgressSet    bool
	flagConsumerProgressForce  bool
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
	consumerProgressCmd.Flags().Uint64Var(&flagConsumerProgressHeight, "height", 0, "set the processed height for the specified consumer progress id (requires --set)")
	consumerProgressCmd.Flags().BoolVar(&flagConsumerProgressSet, "set", false, "set the processed height (requires --id, --height, and --force)")
	consumerProgressCmd.Flags().BoolVar(&flagConsumerProgressForce, "force", false, "confirm setting the processed height")
}

var consumerProgressCmd = &cobra.Command{
	Use:   "consumer-progress",
	Short: "get or set the processed height by consumer progress id",
	RunE: func(cmd *cobra.Command, args []string) error {
		return common.WithStorage(flagDatadir, func(db storage.DB) error {
			if flagConsumerProgressAll && flagConsumerProgressID != "" {
				return fmt.Errorf("cannot use both --id and --all flags")
			}

			// Handle set operation
			if flagConsumerProgressSet {
				if flagConsumerProgressID == "" {
					return fmt.Errorf("--set requires --id to be specified")
				}
				return setConsumerProgress(db, flagConsumerProgressID, flagConsumerProgressHeight, flagConsumerProgressForce)
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

func setConsumerProgress(db storage.DB, progressID string, height uint64, force bool) error {
	// Read existing value first
	var existingHeight uint64
	err := operation.RetrieveProcessedIndex(db.Reader(), progressID, &existingHeight)
	existingFound := err == nil
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("could not retrieve processed index for consumer %s: %w", progressID, err)
	}

	if !force {
		if existingFound {
			return fmt.Errorf("consumer progress %s has existing height %d, refusing to set to %d without --force\n\nTo set, run:\n  read-badger consumer-progress --datadir %s --id %s --set --height %d --force",
				progressID, existingHeight, height, flagDatadir, progressID, height)
		}
		return fmt.Errorf("consumer progress %s not found, refusing to set to %d without --force\n\nTo set, run:\n  read-badger consumer-progress --datadir %s --id %s --set --height %d --force",
			progressID, height, flagDatadir, progressID, height)
	}

	log.Info().Msgf("setting consumer progress for id: %s to height: %d", progressID, height)

	err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return operation.SetProcessedIndex(rw.Writer(), progressID, height)
	})
	if err != nil {
		return fmt.Errorf("could not set processed index for consumer %s: %w", progressID, err)
	}

	fmt.Printf("Consumer Progress ID: %s\n", progressID)
	if existingFound {
		fmt.Printf("Previous Height: %d\n", existingHeight)
	}
	fmt.Printf("Set Processed Height: %d\n", height)

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
