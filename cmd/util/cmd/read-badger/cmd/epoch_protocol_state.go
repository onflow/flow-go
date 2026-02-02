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

func init() {
	rootCmd.AddCommand(epochProtocolStateCmd)

	epochProtocolStateCmd.Flags().StringVarP(&flagBlockID, "block-id", "b", "", "the block id of which to query the protocol state")
	_ = epochProtocolStateCmd.MarkFlagRequired("block-id")
}

var epochProtocolStateCmd = &cobra.Command{
	Use:   "epoch-protocol-state",
	Short: "get epoch protocol state by block ID",
	RunE: func(cmd *cobra.Command, args []string) error {
		return common.WithStorage(flagDatadir, func(db storage.DB) error {
			metrics := metrics.NewNoopCollector()
			setups := store.NewEpochSetups(metrics, db)
			epochCommits := store.NewEpochCommits(metrics, db)
			epochProtocolStateEntries := store.NewEpochProtocolStateEntries(metrics, setups, epochCommits, db,
				store.DefaultEpochProtocolStateCacheSize, store.DefaultProtocolStateIndexCacheSize)

			log.Info().Msgf("got flag block id: %s", flagBlockID)
			blockID, err := flow.HexStringToIdentifier(flagBlockID)
			if err != nil {
				return fmt.Errorf("malformed block id: %w", err)
			}

			log.Info().Msgf("getting protocol state by block id: %v", blockID)
			protocolState, err := epochProtocolStateEntries.ByBlockID(blockID)
			if err != nil {
				return fmt.Errorf("could not get protocol state for block id: %v: %w", blockID, err)
			}

			common.PrettyPrint(protocolState)
			return nil
		})
	},
}
