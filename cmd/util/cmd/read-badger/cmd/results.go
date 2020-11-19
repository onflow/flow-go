package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
)

var flagResultID string

func init() {
	rootCmd.AddCommand(resultsCmd)

	resultsCmd.Flags().StringVar(&flagBlockID, "block-id", "", "the block ID of which to query the result")
	resultsCmd.Flags().StringVar(&flagResultID, "result-id", "", "the ID of the result")
}

var resultsCmd = &cobra.Command{
	Use:   "results",
	Short: "get result by --block-id or --result-id",
	Run: func(cmd *cobra.Command, args []string) {
		storages := InitStorages()

		if flagBlockID != "" {
			blockID, err := flow.HexStringToIdentifier(flagBlockID)
			if err != nil {
				log.Fatal().Err(err).Msg("malformed block ID")
			}

			log.Info().Msgf("getting result by block id: %v", blockID)

			result, err := storages.Results.ByBlockID(blockID)
			if err != nil {
				log.Fatal().Err(err).Msgf("could not get result")
			}

			common.PrettyPrintEntity(result)
			return
		}

		if flagResultID != "" {
			resultID, err := flow.HexStringToIdentifier(flagResultID)
			if err != nil {
				log.Fatal().Err(err).Msg("malformed result ID")
			}

			log.Info().Msgf("getting results by result id: %v", resultID)

			result, err := storages.Results.ByID(resultID)
			if err != nil {
				log.Fatal().Err(err).Msgf("could not get result")
			}

			common.PrettyPrintEntity(result)
			return
		}

		log.Fatal().Msg("missing flags: --block-id or --result-id")
	},
}
