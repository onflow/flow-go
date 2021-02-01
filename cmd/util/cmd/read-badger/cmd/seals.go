package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
)

var flagSealID string

func init() {
	rootCmd.AddCommand(sealsCmd)

	sealsCmd.Flags().StringVarP(&flagSealID, "id", "i", "", "the id of the seal")
	sealsCmd.Flags().StringVarP(&flagBlockID, "block-id", "b", "", "the block id of which to query the seal")
}

var sealsCmd = &cobra.Command{
	Use:   "seals",
	Short: "get seals by block or seal ID",
	Run: func(cmd *cobra.Command, args []string) {
		storages, db := InitStorages()
		defer db.Close()

		if flagSealID != "" && flagBlockID != "" {
			log.Error().Msg("provide one of the flags --id or --block-id")
			return
		}

		if flagSealID != "" {
			log.Info().Msgf("got flag seal id: %s", flagSealID)
			sealID, err := flow.HexStringToIdentifier(flagSealID)
			if err != nil {
				log.Error().Err(err).Msg("malformed seal id")
				return
			}

			log.Info().Msgf("getting seal by id: %v", sealID)
			seal, err := storages.Seals.ByID(sealID)
			if err != nil {
				log.Error().Err(err).Msgf("could not get seal with id: %v", sealID)
				return
			}

			common.PrettyPrintEntity(seal)
			return
		}

		if flagBlockID != "" {
			log.Info().Msgf("got flag block id: %s", flagBlockID)
			blockID, err := flow.HexStringToIdentifier(flagBlockID)
			if err != nil {
				log.Error().Err(err).Msg("malformed block id")
				return
			}

			log.Info().Msgf("getting seal by block id: %v", blockID)
			seal, err := storages.Seals.ByBlockID(blockID)
			if err != nil {
				log.Error().Err(err).Msgf("could not get seal for block id: %v", blockID)
				return
			}

			common.PrettyPrintEntity(seal)
			return
		}

		log.Error().Msg("missing flags --id or --block-id")
	},
}
