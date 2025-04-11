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

var flagResultID string

func init() {
	rootCmd.AddCommand(resultsCmd)

	resultsCmd.Flags().StringVarP(&flagBlockID, "block-id", "b", "", "the block id of which to query the result")
	resultsCmd.Flags().StringVarP(&flagResultID, "id", "i", "", "the id of the result")
}

var resultsCmd = &cobra.Command{
	Use:   "results",
	Short: "get result by block or result ID",
	Run: func(cmd *cobra.Command, args []string) {
		err := WithStorage(func(db storage.DB) error {
			results := store.NewExecutionResults(metrics.NewNoopCollector(), db)
			if flagBlockID != "" {
				log.Info().Msgf("got flag block id: %s", flagBlockID)
				blockID, err := flow.HexStringToIdentifier(flagBlockID)
				if err != nil {
					return fmt.Errorf("malformed block id: %w", err)
				}

				log.Info().Msgf("getting result by block id: %v", blockID)
				result, err := results.ByBlockID(blockID)
				if err != nil {
					return fmt.Errorf("could not get result for block id %v: %w", blockID, err)
				}

				common.PrettyPrintEntity(result)
				// the result does not include the chunk ID, so we need to print it separately
				printChunkID(result)
				return nil
			}

			if flagResultID != "" {
				log.Info().Msgf("got flag result id: %s", flagResultID)
				resultID, err := flow.HexStringToIdentifier(flagResultID)
				if err != nil {
					return fmt.Errorf("malformed result id: %w", err)
				}

				log.Info().Msgf("getting result by id: %v", resultID)
				result, err := results.ByID(resultID)
				if err != nil {
					return fmt.Errorf("could not get result with id %v: %w", resultID, err)
				}

				common.PrettyPrintEntity(result)
				// the result does not include the chunk ID, so we need to print it separately
				printChunkID(result)
				return nil
			}

			return fmt.Errorf("missing flags: --block-id or --result-id")
		})

		if err != nil {
			log.Error().Err(err).Msg("could not get results")
		}
	},
}

func printChunkID(result *flow.ExecutionResult) {
	for i, chunk := range result.Chunks {
		log.Info().Msgf("chunk index %d, id: %v", i, chunk.ID())
	}
}
