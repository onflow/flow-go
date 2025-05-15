package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/state/protocol/protocol_state/kvstore"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
)

var flagDecodeData bool

func init() {
	rootCmd.AddCommand(protocolStateKVStore)

	protocolStateKVStore.Flags().StringVarP(&flagBlockID, "block-id", "b", "", "the block id of which to query the protocol state")
	_ = protocolStateKVStore.MarkFlagRequired("block-id")

	protocolStateKVStore.Flags().BoolVar(&flagDecodeData, "decode", false, "whether to decode the data field")
	_ = protocolStateKVStore.MarkFlagRequired("block-id")
}

var protocolStateKVStore = &cobra.Command{
	Use:   "protocol-kvstore",
	Short: "get protocol state kvstore by block ID",
	Run: func(cmd *cobra.Command, args []string) {
		storages, db := InitStorages()
		defer db.Close()

		log.Info().Msgf("got flag block id: %s", flagBlockID)
		blockID, err := flow.HexStringToIdentifier(flagBlockID)
		if err != nil {
			log.Error().Err(err).Msg("malformed block id")
			return
		}

		log.Info().Msgf("getting protocol state kvstore by block id: %v", blockID)
		protocolState, err := storages.ProtocolKVStore.ByBlockID(blockID)
		if err != nil {
			log.Error().Err(err).Msgf("could not get protocol state kvstore for block id: %v", blockID)
			return
		}
		if !flagDecodeData {
			common.PrettyPrint(protocolState)
			return
		}

		kvstoreAPI, err := kvstore.VersionedDecode(protocolState.Version, protocolState.Data)
		if err != nil {
			log.Error().Err(err).Msgf("could not get protocol state kvstore for block id: %v", blockID)
			return
		}

		var model any
		switch kvstoreAPI.GetProtocolStateVersion() {
		case 0:
			model = kvstoreAPI.(*kvstore.Modelv0)
		case 1:
			model = kvstoreAPI.(*kvstore.Modelv1)
		}
		common.PrettyPrint(model)
	},
}
