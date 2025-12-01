package cmd

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	badgerstate "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/storage"
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
			chainID, err := badgerstate.GetChainIDFromLatestFinalizedHeader(db)
			if err != nil {
				return err
			}
			storages := common.InitStorages(db, chainID) // TODO(4204) - header storage not used

			log.Info().Msgf("got flag block id: %s", flagBlockID)
			blockID, err := flow.HexStringToIdentifier(flagBlockID)
			if err != nil {
				return fmt.Errorf("malformed block id: %w", err)
			}

			log.Info().Msgf("getting protocol state by block id: %v", blockID)
			protocolState, err := storages.EpochProtocolStateEntries.ByBlockID(blockID)
			if err != nil {
				return fmt.Errorf("could not get protocol state for block id: %v: %w", blockID, err)
			}

			common.PrettyPrint(protocolState)
			return nil
		})
	},
}
