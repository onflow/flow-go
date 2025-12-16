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

var flagSealID string

func init() {
	rootCmd.AddCommand(sealsCmd)

	sealsCmd.Flags().StringVarP(&flagSealID, "id", "i", "", "the id of the seal")
	sealsCmd.Flags().StringVarP(&flagBlockID, "block-id", "b", "", "the sealed block id of which to query the seal for")
}

var sealsCmd = &cobra.Command{
	Use:   "seals",
	Short: "get seals by block or seal ID",
	RunE: func(cmd *cobra.Command, args []string) error {
		return common.WithStorage(flagDatadir, func(db storage.DB) error {
			seals := store.NewSeals(&metrics.NoopCollector{}, db)

			if flagSealID != "" && flagBlockID != "" {
				return fmt.Errorf("provide one of the flags --id or --block-id")
			}

			if flagSealID != "" {
				log.Info().Msgf("got flag seal id: %s", flagSealID)
				sealID, err := flow.HexStringToIdentifier(flagSealID)
				if err != nil {
					return fmt.Errorf("malformed seal id: %w", err)
				}

				log.Info().Msgf("getting seal by id: %v", sealID)
				seal, err := seals.ByID(sealID)
				if err != nil {
					return fmt.Errorf("could not get seal with id: %v: %w", sealID, err)
				}

				common.PrettyPrintEntity(seal)
				return nil
			}

			if flagBlockID != "" {
				log.Info().Msgf("got flag block id: %s", flagBlockID)
				blockID, err := flow.HexStringToIdentifier(flagBlockID)
				if err != nil {
					return fmt.Errorf("malformed block id: %w", err)
				}

				log.Info().Msgf("getting seal by block id: %v", blockID)
				seal, err := seals.FinalizedSealForBlock(blockID)
				if err != nil {
					return fmt.Errorf("could not get seal for block id: %v: %w", blockID, err)
				}

				common.PrettyPrintEntity(seal)
				return nil
			}

			return fmt.Errorf("missing flags --id or --block-id")
		})
	},
}
