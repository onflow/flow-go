package cmd

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/state/protocol/protocol_state/kvstore"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
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
	RunE: func(cmd *cobra.Command, args []string) error {
		return common.WithStorage(flagDatadir, func(db storage.DB) error {
			storages := common.InitStorages(db)

			log.Info().Msgf("got flag block id: %s", flagBlockID)
			blockID, err := flow.HexStringToIdentifier(flagBlockID)
			if err != nil {
				return fmt.Errorf("malformed block id: %w", err)
			}

			log.Info().Msgf("getting protocol state kvstore by block id: %v", blockID)
			protocolState, err := storages.ProtocolKVStore.ByBlockID(blockID)
			if err != nil {
				return fmt.Errorf("could not get protocol state kvstore for block id: %v: %w", blockID, err)
			}
			if !flagDecodeData {
				common.PrettyPrint(protocolState)
				return nil
			}

			kvstoreAPI, err := kvstore.VersionedDecode(protocolState.Version, protocolState.Data)
			if err != nil {
				return fmt.Errorf("could not decode protocol state kvstore for block id: %v: %w", blockID, err)
			}

			var model any
			switch kvstoreAPI.GetProtocolStateVersion() {
			case 0:
				model = kvstoreAPI.(*kvstore.Modelv0)
			case 1:
				model = kvstoreAPI.(*kvstore.Modelv1)
			}
			common.PrettyPrint(model)
			return nil
		})
	},
}
