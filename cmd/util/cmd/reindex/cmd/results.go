package cmd

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/storage"
)

func init() {
	rootCmd.AddCommand(resultsCmd)
}

var resultsCmd = &cobra.Command{
	Use:   "results",
	Short: "reindex sealed result IDs by block ID",
	RunE: func(cmd *cobra.Command, args []string) error {
		lockManager := storage.MakeSingletonLockManager()
		return common.WithStorage(flagDatadir, func(db storage.DB) error {
			storages := common.InitStorages(db)
			state, err := common.OpenProtocolState(lockManager, db, storages)
			if err != nil {
				return fmt.Errorf("could not open protocol state: %w", err)
			}

			results := storages.Results
			blocks := storages.Blocks

			root := state.Params().FinalizedRoot()
			final, err := state.Final().Head()
			if err != nil {
				return fmt.Errorf("could not get final header from protocol state: %w", err)
			}

			for h := root.Height + 1; h <= final.Height; h++ {
				block, err := blocks.ByHeight(h)
				if err != nil {
					return fmt.Errorf("could not get block at height %d: %w", h, err)
				}

				for _, seal := range block.Payload.Seals {
					err := results.Index(seal.BlockID, seal.ResultID)
					if err != nil {
						return fmt.Errorf("could not index result ID at height %d: %w", h, err)
					}
				}
			}

			log.Info().Uint64("start_height", root.Height).Uint64("end_height", final.Height).Msg("indexed execution results")
			return nil
		})
	},
}
