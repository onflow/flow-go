package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
)

func init() {
	rootCmd.AddCommand(protocolStateCmd)

	protocolStateCmd.Flags().StringVarP(&flagBlockID, "block-id", "b", "", "the block id of which to query the protocol state")
	_ = protocolStateCmd.MarkFlagRequired("block-id")
}

var protocolStateCmd = &cobra.Command{
	Use:   "protocol-state",
	Short: "get protocol state by block ID",
	Run: func(cmd *cobra.Command, args []string) {
		storages, db := InitStorages()
		defer db.Close()

		log.Info().Msgf("got flag block id: %s", flagBlockID)
		blockID, err := flow.HexStringToIdentifier(flagBlockID)
		if err != nil {
			log.Error().Err(err).Msg("malformed block id")
			return
		}

		log.Info().Msgf("getting protocol state by block id: %v", blockID)
		protocolState, err := storages.ProtocolState.ByBlockID(blockID)
		if err != nil {
			log.Error().Err(err).Msgf("could not get protocol state for block id: %v", blockID)
			return
		}

		common.PrettyPrint(protocolState)
	},
}
