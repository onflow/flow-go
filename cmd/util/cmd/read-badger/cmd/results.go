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

	resultsCmd.Flags().StringVarP(&flagBlockID, "block-id", "b", "", "the block id of which to query the result")
	resultsCmd.Flags().StringVarP(&flagResultID, "id", "i", "", "the id of the result")
}

var resultsCmd = &cobra.Command{
	Use:   "results",
	Short: "get result by block or result ID",
	Run: func(cmd *cobra.Command, args []string) {
		storages, db := InitStorages()
		defer db.Close()

		if flagBlockID != "" {
			log.Info().Msgf("got flag block id: %s", flagBlockID)
			blockID, err := flow.HexStringToIdentifier(flagBlockID)
			if err != nil {
				log.Fatal().Err(err).Msg("malformed block id")
			}

			log.Info().Msgf("getting result by block id: %v", blockID)
			result, err := storages.Results.ByBlockID(blockID)
			if err != nil {
				log.Fatal().Err(err).Msgf("could not get result for block id: %v", blockID)
			}

			common.PrettyPrintEntity(result)
			return
		}

		if flagResultID != "" {
			log.Info().Msgf("got flag result id: %s", flagResultID)
			resultID, err := flow.HexStringToIdentifier(flagResultID)
			if err != nil {
				log.Fatal().Err(err).Msg("malformed result id")
			}

			log.Info().Msgf("getting result by id: %v", resultID)
			result, err := storages.Results.ByID(resultID)
			if err != nil {
				log.Fatal().Err(err).Msgf("could not get result with id: %v", resultID)
			}

			common.PrettyPrintEntity(result)
			return
		}

		log.Fatal().Msg("missing flags: --block-id or --result-id")
	},
}
