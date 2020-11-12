package cmd

import (
	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var flagSealID string

func init() {
	rootCmd.AddCommand(sealsCmd)

	sealsCmd.Flags().StringVarP(&flagSealID, "id", "i", "", "the identifier of the seal")
	sealsCmd.Flags().StringVarP(&flagBlockID, "block-id", "b", "", "the block identifier of which to query the seal")
}

var sealsCmd = &cobra.Command{
	Use:   "seals",
	Short: "get seals by block or seal ID",
	Run: func(cmd *cobra.Command, args []string) {
		storages := InitStorages()

		if flagSealID != "" {
			sealID, err := flow.HexStringToIdentifier(flagSealID)
			if err != nil {
				log.Fatal().Err(err).Msg("malformed seal identifier")
			}

			log.Info().Msgf("getting seal by id: %v", sealID)
			seal, err := storages.Seals.ByID(sealID)
			if err != nil {
				log.Fatal().Err(err).Msgf("could not get seal with identifer: %v", sealID)
			}

			common.PrettyPrintEntity(seal)
			return
		}

		if flagBlockID != "" {
			blockID, err := flow.HexStringToIdentifier(flagBlockID)
			if err != nil {
				log.Fatal().Err(err).Msg("malformed block identifier")
			}

			log.Info().Msgf("getting seal by block id: %v", blockID)
			seal, err := storages.Seals.ByBlockID(blockID)
			if err != nil {
				log.Fatal().Err(err).Msgf("could not get seal for block with identifer: %v", blockID)
			}

			common.PrettyPrintEntity(seal)
			return
		}

		log.Fatal().Msg("missing flags --id or --block-id")
	},
}
